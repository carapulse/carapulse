package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type RunbookCreateRequest struct {
	TenantID string          `json:"tenant_id"`
	Service  string          `json:"service"`
	Name     string          `json:"name"`
	Version  int             `json:"version"`
	Tags     json.RawMessage `json:"tags"`
	Body     string          `json:"body"`
	Spec     json.RawMessage `json:"spec"`
}

func (s *Server) handleRunbooks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodPost:
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		var req RunbookCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Service) == "" || strings.TrimSpace(req.Name) == "" {
			http.Error(w, "service and name required", http.StatusBadRequest)
			return
		}
		tenantID, err := resolveTenant(req.TenantID, r.Header.Get("X-Tenant-Id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req.TenantID = tenantID
		if err := s.policyCheck(r, "runbook.create", "write", ContextRef{TenantID: tenantID}, "low", 0); err != nil {
			s.auditEvent(r.Context(), "runbook.create", "deny", map[string]any{"name": req.Name}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload, err := marshalJSON(req)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		id, err := s.DB.CreateRunbook(r.Context(), payload)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		s.auditEvent(r.Context(), "runbook.create", "allow", map[string]any{"runbook_id": id, "name": req.Name}, "")
		_ = json.NewEncoder(w).Encode(map[string]any{"runbook_id": id})
	case http.MethodGet:
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
		if tenantID == "" {
			http.Error(w, "tenant_id required", http.StatusBadRequest)
			return
		}
		if err := policyCheckTenantRead(s, r, "runbook.list", tenantID); err != nil {
			s.auditEvent(r.Context(), "runbook.list", "deny", map[string]any{}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		out, err := s.DB.ListRunbooks(r.Context())
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		var dbItems []map[string]any
		if err := json.Unmarshal(out, &dbItems); err != nil {
			http.Error(w, "decode error", http.StatusInternalServerError)
			return
		}
		dbItems = filterItemsByTenant(dbItems, tenantID, false)
		if s.WorkspaceDir != "" {
			if fileRunbooks, err := LoadRunbooks(s.WorkspaceDir); err == nil && len(fileRunbooks) > 0 {
				combined := make([]map[string]any, 0, len(dbItems)+len(fileRunbooks))
				seen := map[string]struct{}{}
				for _, item := range dbItems {
					key := runbookKey(item)
					if key != "" {
						seen[key] = struct{}{}
					}
					combined = append(combined, item)
				}
				for _, rb := range fileRunbooks {
					if !tenantMatch(strings.TrimSpace(rb.TenantID), tenantID, false) {
						continue
					}
					item := map[string]any{
						"tenant_id": rb.TenantID,
						"service":   rb.Service,
						"name":      rb.Name,
						"version":   rb.Version,
						"tags":      rb.Tags,
						"body":      rb.Body,
						"spec":      rb.Spec,
					}
					key := runbookKey(item)
					if key != "" {
						if _, ok := seen[key]; ok {
							continue
						}
						seen[key] = struct{}{}
					}
					combined = append(combined, item)
				}
				_ = json.NewEncoder(w).Encode(combined)
				return
			}
		}
		_ = json.NewEncoder(w).Encode(dbItems)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRunbookByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
	if tenantID == "" {
		http.Error(w, "tenant_id required", http.StatusBadRequest)
		return
	}
	runbookID := strings.TrimPrefix(r.URL.Path, "/v1/runbooks/")
	if strings.TrimSpace(runbookID) == "" {
		http.Error(w, "runbook_id required", http.StatusBadRequest)
		return
	}
	if err := policyCheckTenantRead(s, r, "runbook.get", tenantID); err != nil {
		s.auditEvent(r.Context(), "runbook.get", "deny", map[string]any{"runbook_id": runbookID}, err.Error())
		http.Error(w, "policy denied", http.StatusForbidden)
		return
	}
	payload, err := s.DB.GetRunbook(r.Context(), runbookID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	var item map[string]any
	if err := json.Unmarshal(payload, &item); err != nil {
		http.Error(w, "decode error", http.StatusInternalServerError)
		return
	}
	if !tenantMatch(tenantFromMap(item), tenantID, false) {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write(payload)
}

func runbookKey(item map[string]any) string {
	tenant, _ := item["tenant_id"].(string)
	service, _ := item["service"].(string)
	name, _ := item["name"].(string)
	version := 0
	switch v := item["version"].(type) {
	case int:
		version = v
	case float64:
		version = int(v)
	}
	if strings.TrimSpace(service) == "" || strings.TrimSpace(name) == "" {
		return ""
	}
	parts := []string{strings.TrimSpace(service), strings.TrimSpace(name), strconv.Itoa(version)}
	if strings.TrimSpace(tenant) != "" {
		parts = append(parts, strings.TrimSpace(tenant))
	}
	return strings.Join(parts, ":")
}
