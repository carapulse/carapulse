package collectors

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/tools"
)

type fakeRunner struct {
	resp tools.ExecuteResponse
	err  error
	req  tools.ExecuteRequest
}

func (f *fakeRunner) Execute(ctx context.Context, req tools.ExecuteRequest) (tools.ExecuteResponse, error) {
	f.req = req
	return f.resp, f.err
}

func TestSnapshotFromK8sList(t *testing.T) {
	payload := map[string]any{
		"metadata": map[string]any{"resourceVersion": "10"},
		"items": []any{
			map[string]any{
				"kind": "Deployment",
				"metadata": map[string]any{
					"name": "app",
					"namespace": "default",
					"resourceVersion": "11",
				},
			},
		},
	}
	data, _ := json.Marshal(payload)
	nodes, edges, rv := snapshotFromK8sList(data, map[string]string{"env": "dev"})
	if rv != "10" {
		t.Fatalf("rv: %s", rv)
	}
	if len(nodes) == 0 || len(edges) == 0 {
		t.Fatalf("nodes=%d edges=%d", len(nodes), len(edges))
	}
}

func TestSnapshotFromK8sWatch(t *testing.T) {
	line := `{"type":"ADDED","object":{"kind":"Service","metadata":{"name":"svc","namespace":"ns","resourceVersion":"22"}}}`
	snap, rv := snapshotFromK8sWatch([]byte(line), nil)
	if rv != "22" {
		t.Fatalf("rv: %s", rv)
	}
	if len(snap.Nodes) == 0 {
		t.Fatalf("expected nodes")
	}
}

func TestK8sWatcherUsesWatch(t *testing.T) {
	runner := &fakeRunner{resp: tools.ExecuteResponse{Output: []byte(`{"type":"ADDED","object":{"kind":"Pod","metadata":{"name":"p","namespace":"ns","resourceVersion":"5"}}}`)}}
	w := &K8sWatcher{Base: Base{Router: runner}, Resources: []string{"pods"}, Timeout: time.Second, PollInterval: time.Millisecond, SendInitialEvents: true, AllowBookmarks: true}
	ctx, cancel := context.WithCancel(context.Background())
	snapCh := make(chan ctxmodel.Snapshot, 1)
	go func() {
		_ = w.Watch(ctx, snapCh)
	}()
	select {
	case <-snapCh:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("no snapshot")
	}
	cancel()
	if runner.req.Action != "watch" {
		t.Fatalf("action: %s", runner.req.Action)
	}
}

func TestK8sPollerSnapshot(t *testing.T) {
	runner := &fakeRunner{resp: tools.ExecuteResponse{Output: []byte(`{"metadata":{"resourceVersion":"1"},"items":[{"kind":"Deployment","metadata":{"name":"app","namespace":"default","resourceVersion":"2"}}]}`)}}
	p := &K8sPoller{Base: Base{Router: runner}, Resources: []string{"deployments"}}
	snap, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(snap.Nodes) == 0 || len(snap.Edges) == 0 {
		t.Fatalf("nodes=%d edges=%d", len(snap.Nodes), len(snap.Edges))
	}
}

func TestK8sPollerSnapshotRequiresRouter(t *testing.T) {
	p := &K8sPoller{}
	if _, err := p.Snapshot(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}
