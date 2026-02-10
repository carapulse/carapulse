package context

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type watcherStub struct {
	snap Snapshot
	err  error
}

func (w *watcherStub) Watch(ctx context.Context, out chan<- Snapshot) error {
	if w.snap.Nodes != nil || w.snap.Edges != nil {
		out <- w.snap
	}
	return w.err
}

type storeStub struct {
	mu    sync.Mutex
	nodes []Node
	edges []Edge
}

func (s *storeStub) UpsertContextNode(ctx context.Context, node Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes = append(s.nodes, node)
	return nil
}

func (s *storeStub) UpsertContextEdge(ctx context.Context, edge Edge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edges = append(s.edges, edge)
	return nil
}

func (s *storeStub) GetServiceGraph(ctx context.Context, service string) ([]byte, error) {
	return nil, nil
}

func TestContextServiceStartNoStore(t *testing.T) {
	svc := &ContextService{}
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestContextServiceStartDefaults(t *testing.T) {
	store := &storeStub{}
	poller := &fakePoller{snap: Snapshot{Nodes: []Node{{NodeID: "n1"}}}}
	svc := &ContextService{Store: store, Pollers: []Poller{poller}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("err: %v", err)
	}
	if svc.PollInterval != time.Minute {
		t.Fatalf("poll interval: %s", svc.PollInterval)
	}
}

func TestContextServiceRunPollersError(t *testing.T) {
	store := &storeStub{}
	poller := &fakePoller{err: errors.New("poll")}
	svc := &ContextService{Store: store, Pollers: []Poller{poller}, PollInterval: time.Millisecond, Errs: make(chan error, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.runPollers(ctx, make(chan Snapshot))
	select {
	case <-svc.Errs:
		cancel()
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected error")
	}
}

func TestContextServiceRunWatchersError(t *testing.T) {
	svc := &ContextService{Watchers: []Watcher{&watcherStub{err: errors.New("watch")}}, Errs: make(chan error, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.runWatchers(ctx, make(chan Snapshot, 1))
	select {
	case <-svc.Errs:
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected error")
	}
}

func TestContextServiceIngestLoop(t *testing.T) {
	store := &storeStub{}
	svc := &ContextService{Store: store, Errs: make(chan error, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan Snapshot, 1)
	go svc.ingestLoop(ctx, ch)
	ch <- Snapshot{Nodes: []Node{{NodeID: "n1"}}, Edges: []Edge{{EdgeID: "e1"}}}
	time.Sleep(20 * time.Millisecond)
	store.mu.Lock()
	nNodes, nEdges := len(store.nodes), len(store.edges)
	store.mu.Unlock()
	if nNodes != 1 || nEdges != 1 {
		t.Fatalf("nodes=%d edges=%d", nNodes, nEdges)
	}
}

func TestContextServiceSendErr(t *testing.T) {
	svc := &ContextService{}
	svc.sendErr(nil)
	svc.Errs = make(chan error, 1)
	svc.Errs <- errors.New("full")
	svc.sendErr(errors.New("ignored"))
}
