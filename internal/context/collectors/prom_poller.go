package collectors

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/tools"
)

type PromPoller struct {
	Base
	Queries []string
}

func (p *PromPoller) Snapshot(ctx context.Context) (ctxmodel.Snapshot, error) {
	if p.Router == nil {
		return ctxmodel.Snapshot{}, errors.New("router required")
	}
	queries := p.Queries
	if len(queries) == 0 {
		queries = []string{"up"}
	}
	var snap ctxmodel.Snapshot
	for _, query := range queries {
		_, err := p.Router.Execute(ctx, tools.ExecuteRequest{Tool: "prometheus", Action: "query", Input: map[string]any{"query": query}, Context: p.Context})
		if err != nil {
			continue
		}
		id := nodeID("prometheus", "query", hash(query))
		labels := p.withLabels(map[string]string{"query": query})
		snap.Nodes = append(snap.Nodes, node("prometheus.query", id, query, labels))
	}
	return snap, nil
}

func hash(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:8])
}
