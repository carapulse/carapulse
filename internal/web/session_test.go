package web

import (
	"net/http"
	"testing"
)

func TestSessionIDFromRequest(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Session-Id", "sess")
	if got := sessionIDFromRequest(req); got != "sess" {
		t.Fatalf("got: %s", got)
	}
}

func TestEnforceSessionMatch(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Session-Id", "sess")
	plan := map[string]any{"session_id": "sess"}
	if err := enforceSessionMatch(req, plan); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestEnforceSessionMissing(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	plan := map[string]any{"session_id": "sess"}
	if err := enforceSessionMatch(req, plan); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEnforceSessionMismatch(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Session-Id", "a")
	plan := map[string]any{"session_id": "b"}
	if err := enforceSessionMatch(req, plan); err == nil {
		t.Fatalf("expected error")
	}
}
