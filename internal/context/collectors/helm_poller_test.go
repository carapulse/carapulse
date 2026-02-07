package collectors

import (
	"context"
	"encoding/json"
	"testing"

	"carapulse/internal/tools"
)

func TestHelmPollerSnapshot(t *testing.T) {
	rels := []map[string]any{{"name": "svc", "namespace": "default"}}
	data, _ := json.Marshal(rels)
	runner := &fakeRunner{resp: tools.ExecuteResponse{Output: data}}
	poller := &HelmPoller{Base: Base{Router: runner}}
	snap, err := poller.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(snap.Nodes) == 0 {
		t.Fatalf("expected nodes")
	}
}
