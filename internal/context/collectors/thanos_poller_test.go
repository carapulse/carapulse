package collectors

import (
	"context"
	"testing"

	"carapulse/internal/tools"
)

func TestThanosPollerSnapshot(t *testing.T) {
	runner := &fakeRunner{resp: tools.ExecuteResponse{Output: []byte(`{"status":"ok"}`)}}
	poller := &ThanosPoller{Base: Base{Router: runner}, Queries: []string{"up"}}
	snap, err := poller.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if runner.req.Tool != "thanos" || runner.req.Action != "query" {
		t.Fatalf("req: %#v", runner.req)
	}
	if len(snap.Nodes) == 0 {
		t.Fatalf("expected nodes")
	}
}
