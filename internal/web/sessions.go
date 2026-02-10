package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type SessionRequest struct {
	Name         string         `json:"name"`
	TenantID     string         `json:"tenant_id"`
	GroupID      string         `json:"group_id"`
	OwnerActorID string         `json:"owner_actor_id"`
	Metadata     map[string]any `json:"metadata"`
}

type SessionMemberRequest struct {
	MemberID string `json:"member_id"`
	Role     string `json:"role"`
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodPost:
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		var req SessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.TenantID) == "" {
			http.Error(w, "name and tenant_id required", http.StatusBadRequest)
			return
		}
		if err := s.policyCheck(r, "session.create", "write", ContextRef{TenantID: req.TenantID}, "low", 0); err != nil {
			s.auditEvent(r.Context(), "session.create", "deny", map[string]any{"tenant_id": req.TenantID}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload := map[string]any{
			"name":           req.Name,
			"tenant_id":      req.TenantID,
			"group_id":       req.GroupID,
			"owner_actor_id": req.OwnerActorID,
			"metadata":       req.Metadata,
			"created_at":     time.Now().UTC(),
		}
		data, err := marshalJSON(payload)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		id, err := s.DB.CreateSession(r.Context(), data)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		s.auditEvent(r.Context(), "session.create", "allow", map[string]any{"session_id": id}, "")
		_ = json.NewEncoder(w).Encode(map[string]any{"session_id": id})
	case http.MethodGet:
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := s.policyCheckRead(r, "session.list"); err != nil {
			s.auditEvent(r.Context(), "session.list", "deny", map[string]any{}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		limit, offset := parsePagination(r)
		payload, total, err := s.DB.ListSessions(r.Context(), limit, offset)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		tenant := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
		if tenant == "" {
			tenant = strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
		}
		group := strings.TrimSpace(r.URL.Query().Get("group_id"))
		if tenant == "" && group == "" {
			paginatedResponse(w, payload, limit, offset, total)
			return
		}
		filtered, err := filterSessions(payload, tenant, group)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		paginatedResponse(w, filtered, limit, offset, total)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	if strings.HasSuffix(path, "/members") {
		s.handleSessionMembers(w, r)
		return
	}
	sessionID := strings.Trim(path, "/")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}
	if s.DB == nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		if err := s.policyCheckRead(r, "session.get"); err != nil {
			s.auditEvent(r.Context(), "session.get", "deny", map[string]any{"session_id": sessionID}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload, err := s.DB.GetSession(r.Context(), sessionID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if payload == nil {
			http.NotFound(w, r)
			return
		}
		w.Write(payload)
	case http.MethodPut:
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		var req SessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.TenantID) == "" {
			http.Error(w, "name and tenant_id required", http.StatusBadRequest)
			return
		}
		if err := s.policyCheck(r, "session.update", "write", ContextRef{TenantID: req.TenantID}, "low", 0); err != nil {
			s.auditEvent(r.Context(), "session.update", "deny", map[string]any{"session_id": sessionID}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload := map[string]any{
			"name":           req.Name,
			"tenant_id":      req.TenantID,
			"group_id":       req.GroupID,
			"owner_actor_id": req.OwnerActorID,
			"metadata":       req.Metadata,
		}
		data, err := marshalJSON(payload)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		if err := s.DB.UpdateSession(r.Context(), sessionID, data); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		s.auditEvent(r.Context(), "session.update", "allow", map[string]any{"session_id": sessionID}, "")
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := s.policyCheck(r, "session.delete", "write", ContextRef{}, "low", 0); err != nil {
			s.auditEvent(r.Context(), "session.delete", "deny", map[string]any{"session_id": sessionID}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		if err := s.DB.DeleteSession(r.Context(), sessionID); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		s.auditEvent(r.Context(), "session.delete", "allow", map[string]any{"session_id": sessionID}, "")
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSessionMembers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/sessions/"), "/members")
	sessionID := strings.Trim(path, "/")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}
	if s.DB == nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		var req SessionMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.MemberID) == "" || strings.TrimSpace(req.Role) == "" {
			http.Error(w, "member_id and role required", http.StatusBadRequest)
			return
		}
		if err := s.policyCheck(r, "session.member.add", "write", ContextRef{}, "low", 0); err != nil {
			s.auditEvent(r.Context(), "session.member.add", "deny", map[string]any{"session_id": sessionID}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload := map[string]any{"member_id": req.MemberID, "role": req.Role}
		data, err := marshalJSON(payload)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		if err := s.DB.AddSessionMember(r.Context(), sessionID, data); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		s.auditEvent(r.Context(), "session.member.add", "allow", map[string]any{"session_id": sessionID, "member_id": req.MemberID}, "")
		w.WriteHeader(http.StatusNoContent)
	case http.MethodGet:
		if err := s.policyCheckRead(r, "session.member.list"); err != nil {
			s.auditEvent(r.Context(), "session.member.list", "deny", map[string]any{"session_id": sessionID}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload, err := s.DB.ListSessionMembers(r.Context(), sessionID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		w.Write(payload)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func filterSessions(payload []byte, tenantID, groupID string) ([]byte, error) {
	var items []map[string]any
	if err := json.Unmarshal(payload, &items); err != nil {
		return payload, err
	}
	tenantID = strings.TrimSpace(tenantID)
	groupID = strings.TrimSpace(groupID)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if tenantID != "" {
			if v, ok := item["tenant_id"].(string); !ok || v != tenantID {
				continue
			}
		}
		if groupID != "" {
			if v, ok := item["group_id"].(string); !ok || v != groupID {
				continue
			}
		}
		out = append(out, item)
	}
	return json.Marshal(out)
}
