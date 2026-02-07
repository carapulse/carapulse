package collectors

import (
	"context"
	"testing"

	"carapulse/internal/tools"
)

func TestPromPollerSnapshot(t *testing.T) {
	runner := &fakeRunner{resp: tools.ExecuteResponse{Output: []byte(`{"status":"success"}`)}}
	poller := &PromPoller{Base: Base{Router: runner}, Queries: []string{"up"}}
	snap, err := poller.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(snap.Nodes) == 0 {
		t.Fatalf("expected nodes")
	}
}
