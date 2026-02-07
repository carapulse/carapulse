package collectors

import (
	"context"
	"time"

	ctxmodel "carapulse/internal/context"
)

type PollingWatcher struct {
	Poller   ctxmodel.Poller
	Interval time.Duration
	Last     ctxmodel.Snapshot
	DiffFn   func(prev, next ctxmodel.Snapshot) ctxmodel.Snapshot
}

func (w *PollingWatcher) Watch(ctx context.Context, out chan<- ctxmodel.Snapshot) error {
	if w == nil || w.Poller == nil {
		return nil
	}
	if w.Interval <= 0 {
		w.Interval = 30 * time.Second
	}
	diff := w.DiffFn
	if diff == nil {
		diff = diffSnapshot
	}
	for {
		snap, err := w.Poller.Snapshot(ctx)
		if err == nil {
			update := diff(w.Last, snap)
			if len(update.Nodes) > 0 || len(update.Edges) > 0 {
				out <- update
			}
			w.Last = snap
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(w.Interval):
		}
	}
}

func diffSnapshot(prev, next ctxmodel.Snapshot) ctxmodel.Snapshot {
	prevNodes := map[string]struct{}{}
	prevEdges := map[string]struct{}{}
	for _, node := range prev.Nodes {
		if node.NodeID != "" {
			prevNodes[node.NodeID] = struct{}{}
		}
	}
	for _, edge := range prev.Edges {
		if edge.EdgeID != "" {
			prevEdges[edge.EdgeID] = struct{}{}
		}
	}
	var out ctxmodel.Snapshot
	for _, node := range next.Nodes {
		if node.NodeID == "" {
			continue
		}
		if _, ok := prevNodes[node.NodeID]; ok {
			continue
		}
		out.Nodes = append(out.Nodes, node)
	}
	for _, edge := range next.Edges {
		if edge.EdgeID == "" {
			continue
		}
		if _, ok := prevEdges[edge.EdgeID]; ok {
			continue
		}
		out.Edges = append(out.Edges, edge)
	}
	return out
}
