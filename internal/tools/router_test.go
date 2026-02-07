package tools

import "testing"

func TestEnsureCLINotFound(t *testing.T) {
	r := NewRouter()
	if err := r.EnsureCLI("definitely-not-a-command"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEnsureCLISuccess(t *testing.T) {
	r := NewRouter()
	// "sh" should exist on unix, skip on windows.
	if err := r.EnsureCLI("sh"); err != nil {
		t.Skipf("cli not found: %v", err)
	}
}

func TestNewRouterLogs(t *testing.T) {
	r := NewRouter()
	if r.Logs == nil {
		t.Fatalf("expected log hub")
	}
}

func TestLogHubInit(t *testing.T) {
	r := &Router{}
	if r.logHub() == nil {
		t.Fatalf("expected log hub")
	}
}

func TestLogHubNilRouter(t *testing.T) {
	var r *Router
	if r.logHub() != nil {
		t.Fatalf("expected nil")
	}
}
