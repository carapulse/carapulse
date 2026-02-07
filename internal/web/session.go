package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

func sessionIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.Header.Get("X-Session-Id"))
}

func sessionFromPlan(plan map[string]any) string {
	if plan == nil {
		return ""
	}
	if v, ok := plan["session_id"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func enforceSessionMatch(req *http.Request, plan map[string]any) error {
	planSession := sessionFromPlan(plan)
	if planSession == "" {
		return nil
	}
	reqSession := sessionIDFromRequest(req)
	if reqSession == "" {
		return errors.New("session required")
	}
	if planSession != reqSession {
		return errors.New("session mismatch")
	}
	return nil
}

type SessionMemberChecker interface {
	IsSessionMember(ctx context.Context, sessionID, memberID string) (bool, error)
}

func (s *Server) requireSession(r *http.Request) (string, error) {
	if s == nil || !s.SessionRequired {
		return "", nil
	}
	sessionID := sessionIDFromRequest(r)
	if sessionID == "" {
		return "", errors.New("session required")
	}
	actor, ok := ActorFromContext(r.Context())
	if !ok || strings.TrimSpace(actor.ID) == "" {
		return sessionID, nil
	}
	checker, ok := any(s.DB).(SessionMemberChecker)
	if !ok {
		return sessionID, nil
	}
	allowed, err := checker.IsSessionMember(r.Context(), sessionID, actor.ID)
	if err != nil {
		return "", err
	}
	if !allowed {
		return "", errors.New("session unauthorized")
	}
	return sessionID, nil
}
