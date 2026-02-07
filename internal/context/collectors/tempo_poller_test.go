package collectors

import (
	"context"
	"testing"

	"carapulse/internal/tools"
)

func TestTempoPollerSnapshot(t *testing.T) {
	runner := &fakeRunner{resp: tools.ExecuteResponse{Output: []byte(`{"data":[]}`)}}
	poller := &TempoPoller{Base: Base{Router: runner}, Queries: []string{"service=api"}}
	snap, err := poller.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(snap.Nodes) == 0 {
		t.Fatalf("expected nodes")
	}
}
