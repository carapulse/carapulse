package collectors

import (
	"context"
	"errors"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/tools"
)

type ThanosPoller struct {
	Base
	Queries []string
}

func (p *ThanosPoller) Snapshot(ctx context.Context) (ctxmodel.Snapshot, error) {
	if p.Router == nil {
		return ctxmodel.Snapshot{}, errors.New("router required")
	}
	queries := p.Queries
	if len(queries) == 0 {
		queries = []string{"up"}
	}
	var snap ctxmodel.Snapshot
	for _, query := range queries {
		_, err := p.Router.Execute(ctx, tools.ExecuteRequest{Tool: "thanos", Action: "query", Input: map[string]any{"query": query}, Context: p.Context})
		if err != nil {
			continue
		}
		id := nodeID("thanos", "query", hash(query))
		labels := p.withLabels(map[string]string{"query": query})
		snap.Nodes = append(snap.Nodes, node("thanos.query", id, query, labels))
	}
	return snap, nil
}
