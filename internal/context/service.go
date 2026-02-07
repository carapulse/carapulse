package context

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

type Service struct {
	ServiceID string
	Name      string
	OwnerTeam string
}

type ServiceGraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Snapshot struct {
	Nodes []Node
	Edges []Edge
}

type Node struct {
	NodeID    string            `json:"node_id"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels"`
	OwnerTeam string            `json:"owner_team"`
}

type Edge struct {
	EdgeID     string `json:"edge_id"`
	FromNodeID string `json:"from_node_id"`
	ToNodeID   string `json:"to_node_id"`
	Relation   string `json:"relation"`
}

type Store interface {
	UpsertContextNode(ctx context.Context, node Node) error
	UpsertContextEdge(ctx context.Context, edge Edge) error
	GetServiceGraph(ctx context.Context, service string) ([]byte, error)
}

type SnapshotWriter interface {
	InsertContextSnapshot(ctx context.Context, source string, nodesJSON, edgesJSON, labelsJSON []byte) (string, error)
}

type Poller interface {
	Snapshot(ctx context.Context) (Snapshot, error)
}

type Watcher interface {
	Watch(ctx context.Context, out chan<- Snapshot) error
}

type ContextService struct {
	Store            Store
	SnapshotWriter   SnapshotWriter
	Pollers          []Poller
	Watchers         []Watcher
	PollInterval     time.Duration
	SnapshotInterval time.Duration
	Errs             chan error
	lastSnapshot     time.Time
}

func New() *ContextService {
	return &ContextService{}
}

func NewWithStore(store Store) *ContextService {
	return &ContextService{Store: store}
}

func NewWithStoreAndPollers(store Store, pollers ...Poller) *ContextService {
	return &ContextService{Store: store, Pollers: pollers}
}

func (s *ContextService) Start(ctx context.Context) error {
	if s.Store == nil {
		return nil
	}
	if s.PollInterval <= 0 {
		s.PollInterval = time.Minute
	}
	if s.SnapshotInterval <= 0 {
		s.SnapshotInterval = 5 * time.Minute
	}
	snapshots := make(chan Snapshot, 8)
	go s.runPollers(ctx, snapshots)
	go s.runWatchers(ctx, snapshots)
	go s.ingestLoop(ctx, snapshots)
	return nil
}

func (s *ContextService) runPollers(ctx context.Context, out chan<- Snapshot) {
	if len(s.Pollers) == 0 {
		return
	}
	ticker := time.NewTicker(s.PollInterval)
	defer ticker.Stop()
	for {
		if err := s.RefreshContext(ctx); err != nil {
			s.sendErr(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *ContextService) runWatchers(ctx context.Context, out chan<- Snapshot) {
	if len(s.Watchers) == 0 {
		return
	}
	for _, watcher := range s.Watchers {
		if watcher == nil {
			continue
		}
		go func(w Watcher) {
			if err := w.Watch(ctx, out); err != nil && ctx.Err() == nil {
				s.sendErr(err)
			}
		}(watcher)
	}
}

func (s *ContextService) ingestLoop(ctx context.Context, in <-chan Snapshot) {
	for {
		select {
		case <-ctx.Done():
			return
		case snap := <-in:
			if err := s.IngestSnapshot(ctx, snap.Nodes, snap.Edges); err != nil {
				s.sendErr(err)
			}
			s.maybeSnapshot(ctx, snap)
		}
	}
}

func (s *ContextService) maybeSnapshot(ctx context.Context, snap Snapshot) {
	if s.SnapshotWriter == nil {
		return
	}
	if len(snap.Nodes) == 0 && len(snap.Edges) == 0 {
		return
	}
	now := time.Now().UTC()
	if !s.lastSnapshot.IsZero() && now.Sub(s.lastSnapshot) < s.SnapshotInterval {
		return
	}
	nodesJSON, err := json.Marshal(snap.Nodes)
	if err != nil {
		return
	}
	edgesJSON, err := json.Marshal(snap.Edges)
	if err != nil {
		return
	}
	var labelsJSON []byte
	if labels := snapshotLabels(snap.Nodes); len(labels) > 0 {
		if data, err := json.Marshal(labels); err == nil {
			labelsJSON = data
		}
	}
	if _, err := s.SnapshotWriter.InsertContextSnapshot(ctx, "context", nodesJSON, edgesJSON, labelsJSON); err == nil {
		s.lastSnapshot = now
	}
}

func snapshotLabels(nodes []Node) map[string]string {
	if len(nodes) == 0 {
		return nil
	}
	keys := []string{
		"tenant_id",
		"environment",
		"cluster_id",
		"namespace",
		"aws_account_id",
		"region",
		"argocd_project",
		"grafana_org_id",
	}
	out := map[string]string{}
	for _, key := range keys {
		val := ""
		consistent := true
		for _, node := range nodes {
			if node.Labels == nil {
				continue
			}
			if label, ok := node.Labels[key]; ok && strings.TrimSpace(label) != "" {
				if val == "" {
					val = label
				} else if val != label {
					consistent = false
					break
				}
			}
		}
		if consistent && val != "" {
			out[key] = val
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *ContextService) sendErr(err error) {
	if err == nil {
		return
	}
	if s.Errs == nil {
		return
	}
	select {
	case s.Errs <- err:
	default:
	}
}

func (s *ContextService) RefreshContext(ctx context.Context) error {
	if s.Store == nil || len(s.Pollers) == 0 {
		return nil
	}
	for _, poller := range s.Pollers {
		if poller == nil {
			continue
		}
		snap, err := poller.Snapshot(ctx)
		if err != nil {
			return err
		}
		if err := s.IngestSnapshot(ctx, snap.Nodes, snap.Edges); err != nil {
			return err
		}
	}
	return nil
}

func (s *ContextService) IngestSnapshot(ctx context.Context, nodes []Node, edges []Edge) error {
	if s.Store == nil {
		return nil
	}
	for _, node := range nodes {
		if err := s.Store.UpsertContextNode(ctx, node); err != nil {
			return err
		}
	}
	for _, edge := range edges {
		if err := s.Store.UpsertContextEdge(ctx, edge); err != nil {
			return err
		}
	}
	return nil
}

func (s *ContextService) GetServiceGraph(ctx context.Context, service string) (ServiceGraph, error) {
	if s.Store == nil {
		return ServiceGraph{}, nil
	}
	data, err := s.Store.GetServiceGraph(ctx, service)
	if err != nil {
		return ServiceGraph{}, err
	}
	if len(data) == 0 {
		return ServiceGraph{}, nil
	}
	var graph ServiceGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return ServiceGraph{}, err
	}
	return graph, nil
}
