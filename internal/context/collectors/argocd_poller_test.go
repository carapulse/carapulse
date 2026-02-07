package collectors

import (
	"context"
	"encoding/json"
	"testing"

	"carapulse/internal/tools"
)

func TestArgoCDPollerSnapshot(t *testing.T) {
	apps := []map[string]any{{
		"metadata": map[string]any{"name": "app"},
		"spec": map[string]any{
			"destination": map[string]any{"namespace": "default"},
		},
	}}
	data, _ := json.Marshal(apps)
	runner := &fakeRunner{resp: tools.ExecuteResponse{Output: data}}
	poller := &ArgoCDPoller{Base: Base{Router: runner}}
	snap, err := poller.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(snap.Nodes) == 0 {
		t.Fatalf("expected nodes")
	}
}
