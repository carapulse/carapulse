package workflows

import (
	"context"
	"testing"

	"carapulse/internal/tools"
)

func TestRunCLIError(t *testing.T) {
	rt := NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})
	if _, err := rt.RunCLI(context.Background(), "unknown", "status", nil); err == nil {
		t.Fatalf("expected error")
	}
}
