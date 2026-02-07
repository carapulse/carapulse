package context

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	nodes    []Node
	edges    []Edge
	nodeErr  error
	edgeErr  error
	graph    []byte
	graphErr error
}

type fakePoller struct {
	snap  Snapshot
	err   error
	calls int
}

func (f *fakePoller) Snapshot(ctx context.Context) (Snapshot, error) {
	f.calls++
	return f.snap, f.err
}

func (f *fakeStore) UpsertContextNode(ctx context.Context, node Node) error {
	f.nodes = append(f.nodes, node)
	return f.nodeErr
}

func (f *fakeStore) UpsertContextEdge(ctx context.Context, edge Edge) error {
	f.edges = append(f.edges, edge)
	return f.edgeErr
}

func (f *fakeStore) GetServiceGraph(ctx context.Context, service string) ([]byte, error) {
	return f.graph, f.graphErr
}

func TestContextServiceNoop(t *testing.T) {
	svc := New()
	if err := svc.RefreshContext(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := svc.IngestSnapshot(context.Background(), nil, nil); err != nil {
		t.Fatalf("err: %v", err)
	}
	graph, err := svc.GetServiceGraph(context.Background(), "svc")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(graph.Nodes) != 0 || len(graph.Edges) != 0 {
		t.Fatalf("unexpected graph")
	}
}

func TestContextServiceRefreshNoPollers(t *testing.T) {
	store := &fakeStore{}
	svc := NewWithStore(store)
	if err := svc.RefreshContext(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(store.nodes) != 0 || len(store.edges) != 0 {
		t.Fatalf("unexpected ingest")
	}
}

func TestContextServiceRefreshNilPoller(t *testing.T) {
	store := &fakeStore{}
	poller := &fakePoller{snap: Snapshot{Nodes: []Node{{NodeID: "n1"}}, Edges: []Edge{{EdgeID: "e1"}}}}
	svc := NewWithStoreAndPollers(store, nil, poller)
	if err := svc.RefreshContext(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if poller.calls != 1 {
		t.Fatalf("calls: %d", poller.calls)
	}
	if len(store.nodes) != 1 || len(store.edges) != 1 {
		t.Fatalf("nodes=%d edges=%d", len(store.nodes), len(store.edges))
	}
}

func TestContextServiceRefreshPollerError(t *testing.T) {
	store := &fakeStore{}
	poller := &fakePoller{err: errors.New("poll")}
	svc := NewWithStoreAndPollers(store, poller)
	if err := svc.RefreshContext(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
	if poller.calls != 1 {
		t.Fatalf("calls: %d", poller.calls)
	}
	if len(store.nodes) != 0 || len(store.edges) != 0 {
		t.Fatalf("unexpected ingest")
	}
}

func TestContextServiceRefreshIngestError(t *testing.T) {
	store := &fakeStore{nodeErr: errors.New("node")}
	poller := &fakePoller{snap: Snapshot{Nodes: []Node{{NodeID: "n1"}}}}
	svc := NewWithStoreAndPollers(store, poller)
	if err := svc.RefreshContext(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
	if poller.calls != 1 {
		t.Fatalf("calls: %d", poller.calls)
	}
}

func TestContextServiceRefreshOK(t *testing.T) {
	store := &fakeStore{}
	poller := &fakePoller{snap: Snapshot{
		Nodes: []Node{{NodeID: "n1"}, {NodeID: "n2"}},
		Edges: []Edge{{EdgeID: "e1"}},
	}}
	svc := NewWithStoreAndPollers(store, poller)
	if err := svc.RefreshContext(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if poller.calls != 1 {
		t.Fatalf("calls: %d", poller.calls)
	}
	if len(store.nodes) != 2 || len(store.edges) != 1 {
		t.Fatalf("nodes=%d edges=%d", len(store.nodes), len(store.edges))
	}
}

func TestContextServiceIngestSnapshotNodeError(t *testing.T) {
	store := &fakeStore{nodeErr: errors.New("node")}
	svc := NewWithStore(store)
	if err := svc.IngestSnapshot(context.Background(), []Node{{NodeID: "n1"}}, []Edge{{EdgeID: "e1"}}); err == nil {
		t.Fatalf("expected error")
	}
	if len(store.edges) != 0 {
		t.Fatalf("edges should be empty")
	}
}

func TestContextServiceIngestSnapshotEdgeError(t *testing.T) {
	store := &fakeStore{edgeErr: errors.New("edge")}
	svc := NewWithStore(store)
	if err := svc.IngestSnapshot(context.Background(), []Node{{NodeID: "n1"}}, []Edge{{EdgeID: "e1"}}); err == nil {
		t.Fatalf("expected error")
	}
	if len(store.nodes) != 1 {
		t.Fatalf("nodes: %d", len(store.nodes))
	}
}

func TestContextServiceIngestSnapshotOK(t *testing.T) {
	store := &fakeStore{}
	svc := NewWithStore(store)
	nodes := []Node{{NodeID: "n1"}, {NodeID: "n2"}}
	edges := []Edge{{EdgeID: "e1"}, {EdgeID: "e2"}}
	if err := svc.IngestSnapshot(context.Background(), nodes, edges); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(store.nodes) != 2 || len(store.edges) != 2 {
		t.Fatalf("nodes=%d edges=%d", len(store.nodes), len(store.edges))
	}
}

func TestContextServiceGetServiceGraphError(t *testing.T) {
	store := &fakeStore{graphErr: errors.New("graph")}
	svc := NewWithStore(store)
	if _, err := svc.GetServiceGraph(context.Background(), "svc"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestContextServiceGetServiceGraphEmpty(t *testing.T) {
	store := &fakeStore{}
	svc := NewWithStore(store)
	graph, err := svc.GetServiceGraph(context.Background(), "svc")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(graph.Nodes) != 0 || len(graph.Edges) != 0 {
		t.Fatalf("unexpected graph")
	}
}

func TestContextServiceGetServiceGraphInvalidJSON(t *testing.T) {
	store := &fakeStore{graph: []byte("{")}
	svc := NewWithStore(store)
	if _, err := svc.GetServiceGraph(context.Background(), "svc"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestContextServiceGetServiceGraphOK(t *testing.T) {
	store := &fakeStore{graph: []byte(`{"nodes":[{"node_id":"n1","kind":"service","name":"svc","labels":{"env":"prod"},"owner_team":"team"}],"edges":[{"edge_id":"e1","from_node_id":"n1","to_node_id":"n2","relation":"calls"}]}`)}
	svc := NewWithStore(store)
	graph, err := svc.GetServiceGraph(context.Background(), "svc")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(graph.Nodes) != 1 || len(graph.Edges) != 1 {
		t.Fatalf("nodes=%d edges=%d", len(graph.Nodes), len(graph.Edges))
	}
	if graph.Nodes[0].OwnerTeam != "team" || graph.Edges[0].Relation != "calls" {
		t.Fatalf("unexpected graph data")
	}
}

func TestSnapshotLabels(t *testing.T) {
	nodes := []Node{
		{Labels: map[string]string{"tenant_id": "t", "environment": "dev"}},
		{Labels: map[string]string{"tenant_id": "t", "environment": "dev"}},
	}
	labels := snapshotLabels(nodes)
	if labels["tenant_id"] != "t" || labels["environment"] != "dev" {
		t.Fatalf("labels: %#v", labels)
	}
	nodes = []Node{
		{Labels: map[string]string{"tenant_id": "t"}},
		{Labels: map[string]string{"tenant_id": "other"}},
	}
	labels = snapshotLabels(nodes)
	if _, ok := labels["tenant_id"]; ok {
		t.Fatalf("expected tenant_id omitted: %#v", labels)
	}
}
