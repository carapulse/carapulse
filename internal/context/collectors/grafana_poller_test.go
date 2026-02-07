package collectors

import (
	"context"
	"encoding/json"
	"testing"

	"carapulse/internal/tools"
)

func TestGrafanaPollerSnapshot(t *testing.T) {
	items := []map[string]any{{"uid": "dash", "title": "Dash", "folderId": 1}}
	data, _ := json.Marshal(items)
	runner := &fakeRunner{resp: tools.ExecuteResponse{Output: data}}
	poller := &GrafanaPoller{Base: Base{Router: runner}}
	snap, err := poller.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(snap.Nodes) == 0 {
		t.Fatalf("expected nodes")
	}
}
