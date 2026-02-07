package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handlePlaybooks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodPost:
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		var req PlaybookCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		tenantID, err := resolveTenant(req.TenantID, r.Header.Get("X-Tenant-Id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Version == 0 {
			req.Version = 1
		}
		if req.Spec == nil {
			http.Error(w, "spec required", http.StatusBadRequest)
			return
		}
		req.TenantID = tenantID
		if err := s.policyCheck(r, "playbook.create", "write", ContextRef{TenantID: tenantID}, "low", 0); err != nil {
			s.auditEvent(r.Context(), "playbook.create", "deny", map[string]any{"name": req.Name}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload := map[string]any{
			"tenant_id":  req.TenantID,
			"name":       req.Name,
			"version":    req.Version,
			"tags":       req.Tags,
			"spec":       req.Spec,
			"created_at": time.Now().UTC(),
		}
		data, err := marshalJSON(payload)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		id, err := s.DB.CreatePlaybook(r.Context(), data)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		s.auditEvent(r.Context(), "playbook.create", "allow", map[string]any{"playbook_id": id, "name": req.Name}, "")
		_ = json.NewEncoder(w).Encode(map[string]any{"playbook_id": id})
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
		if err := policyCheckTenantRead(s, r, "playbook.list", tenantID); err != nil {
			s.auditEvent(r.Context(), "playbook.list", "deny", map[string]any{}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload, err := s.DB.ListPlaybooks(r.Context())
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		filtered, err := filterJSONByTenant(payload, tenantID, false)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		w.Write(filtered)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePlaybookByID(w http.ResponseWriter, r *http.Request) {
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
	playbookID := strings.TrimPrefix(r.URL.Path, "/v1/playbooks/")
	if strings.TrimSpace(playbookID) == "" {
		http.Error(w, "playbook_id required", http.StatusBadRequest)
		return
	}
	if err := policyCheckTenantRead(s, r, "playbook.get", tenantID); err != nil {
		s.auditEvent(r.Context(), "playbook.get", "deny", map[string]any{"playbook_id": playbookID}, err.Error())
		http.Error(w, "policy denied", http.StatusForbidden)
		return
	}
	payload, err := s.DB.GetPlaybook(r.Context(), playbookID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if payload == nil {
		http.NotFound(w, r)
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
	w.Write(payload)
}
