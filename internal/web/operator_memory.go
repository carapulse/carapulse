package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type OperatorMemoryStore interface {
	CreateOperatorMemory(ctx context.Context, payload []byte) (string, error)
	ListOperatorMemory(ctx context.Context, tenantID string) ([]byte, error)
	GetOperatorMemory(ctx context.Context, memoryID string) ([]byte, error)
	UpdateOperatorMemory(ctx context.Context, memoryID string, payload []byte) error
	DeleteOperatorMemory(ctx context.Context, memoryID string) error
}

type OperatorMemoryRequest struct {
	TenantID     string         `json:"tenant_id"`
	Title        string         `json:"title"`
	Body         string         `json:"body"`
	Tags         []string       `json:"tags"`
	Metadata     map[string]any `json:"metadata"`
	OwnerActorID string         `json:"owner_actor_id"`
}

func (s *Server) handleOperatorMemory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	store, ok := s.DB.(OperatorMemoryStore)
	if !ok || store == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		var req OperatorMemoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		tenant, err := resolveTenant(req.TenantID, r.Header.Get("X-Tenant-Id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Body) == "" {
			http.Error(w, "title and body required", http.StatusBadRequest)
			return
		}
		if err := s.policyCheck(r, "memory.create", "write", ContextRef{TenantID: tenant}, "low", 0); err != nil {
			s.auditEvent(r.Context(), "memory.create", "deny", map[string]any{"tenant_id": tenant}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload := map[string]any{
			"tenant_id":      tenant,
			"title":          req.Title,
			"body":           req.Body,
			"tags":           req.Tags,
			"metadata":       req.Metadata,
			"owner_actor_id": req.OwnerActorID,
			"created_at":     time.Now().UTC(),
		}
		data, err := marshalJSON(payload)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		id, err := store.CreateOperatorMemory(r.Context(), data)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		s.auditEvent(r.Context(), "memory.create", "allow", map[string]any{"memory_id": id, "tenant_id": tenant}, "")
		_ = json.NewEncoder(w).Encode(map[string]any{"memory_id": id})
	case http.MethodGet:
		tenant := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
		if tenant == "" {
			tenant = strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
		}
		if tenant == "" {
			http.Error(w, "tenant_id required", http.StatusBadRequest)
			return
		}
		if err := policyCheckTenantRead(s, r, "memory.list", tenant); err != nil {
			s.auditEvent(r.Context(), "memory.list", "deny", map[string]any{"tenant_id": tenant}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload, err := store.ListOperatorMemory(r.Context(), tenant)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		var dbItems []map[string]any
		if err := json.Unmarshal(payload, &dbItems); err != nil {
			http.Error(w, "decode error", http.StatusInternalServerError)
			return
		}
		if s.WorkspaceDir != "" {
			if fileEntries, err := LoadOperatorMemory(s.WorkspaceDir); err == nil && len(fileEntries) > 0 {
				combined := make([]map[string]any, 0, len(dbItems)+len(fileEntries))
				seen := map[string]struct{}{}
				for _, item := range dbItems {
					key := strings.TrimSpace(tenantFromMap(item)) + ":" + strings.TrimSpace(stringFromMap(item, "title"))
					if key != ":" {
						seen[key] = struct{}{}
					}
					combined = append(combined, item)
				}
				for _, entry := range fileEntries {
					if !tenantMatch(entry.TenantID, tenant, false) {
						continue
					}
					key := strings.TrimSpace(entry.TenantID) + ":" + strings.TrimSpace(entry.Title)
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					item := map[string]any{
						"memory_id":      fileMemoryID(entry),
						"tenant_id":      entry.TenantID,
						"title":          entry.Title,
						"body":           entry.Body,
						"tags":           entry.Tags,
						"metadata":       entry.Metadata,
						"owner_actor_id": entry.OwnerActorID,
						"source":         "file",
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

func (s *Server) handleOperatorMemoryByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	store, ok := s.DB.(OperatorMemoryStore)
	if !ok || store == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	memoryID := strings.TrimPrefix(r.URL.Path, "/v1/memory/")
	memoryID = strings.Trim(memoryID, "/")
	if memoryID == "" {
		http.Error(w, "memory_id required", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
		if tenantID == "" {
			http.Error(w, "tenant_id required", http.StatusBadRequest)
			return
		}
		if err := policyCheckTenantRead(s, r, "memory.get", tenantID); err != nil {
			s.auditEvent(r.Context(), "memory.get", "deny", map[string]any{"memory_id": memoryID}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload, err := store.GetOperatorMemory(r.Context(), memoryID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if payload != nil {
			var item map[string]any
			if err := json.Unmarshal(payload, &item); err != nil {
				http.Error(w, "decode error", http.StatusInternalServerError)
				return
			}
			if tenantMatch(tenantFromMap(item), tenantID, false) {
				_, _ = w.Write(payload)
				return
			}
		}
		if s.WorkspaceDir != "" {
			if fileEntries, err := LoadOperatorMemory(s.WorkspaceDir); err == nil && len(fileEntries) > 0 {
				for _, entry := range fileEntries {
					if !tenantMatch(entry.TenantID, tenantID, false) {
						continue
					}
					if fileMemoryID(entry) == memoryID {
						item := map[string]any{
							"memory_id":      memoryID,
							"tenant_id":      entry.TenantID,
							"title":          entry.Title,
							"body":           entry.Body,
							"tags":           entry.Tags,
							"metadata":       entry.Metadata,
							"owner_actor_id": entry.OwnerActorID,
							"source":         "file",
						}
						_ = json.NewEncoder(w).Encode(item)
						return
					}
				}
			}
		}
		http.NotFound(w, r)
	case http.MethodPut:
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		var req OperatorMemoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		tenant, err := resolveTenant(req.TenantID, r.Header.Get("X-Tenant-Id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Body) == "" {
			http.Error(w, "title and body required", http.StatusBadRequest)
			return
		}
		if err := s.policyCheck(r, "memory.update", "write", ContextRef{TenantID: tenant}, "low", 0); err != nil {
			s.auditEvent(r.Context(), "memory.update", "deny", map[string]any{"memory_id": memoryID, "tenant_id": tenant}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload := map[string]any{
			"tenant_id":      tenant,
			"title":          req.Title,
			"body":           req.Body,
			"tags":           req.Tags,
			"metadata":       req.Metadata,
			"owner_actor_id": req.OwnerActorID,
		}
		data, err := marshalJSON(payload)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		if err := store.UpdateOperatorMemory(r.Context(), memoryID, data); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		s.auditEvent(r.Context(), "memory.update", "allow", map[string]any{"memory_id": memoryID, "tenant_id": tenant}, "")
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		tenant := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
		if tenant == "" {
			http.Error(w, "tenant_id required", http.StatusBadRequest)
			return
		}
		if err := s.policyCheck(r, "memory.delete", "write", ContextRef{TenantID: tenant}, "low", 0); err != nil {
			s.auditEvent(r.Context(), "memory.delete", "deny", map[string]any{"memory_id": memoryID}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		if err := store.DeleteOperatorMemory(r.Context(), memoryID); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		s.auditEvent(r.Context(), "memory.delete", "allow", map[string]any{"memory_id": memoryID}, "")
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
