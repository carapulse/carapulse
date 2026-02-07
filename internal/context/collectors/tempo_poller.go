package collectors

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/tools"
)

type TempoPoller struct {
	Base
	Queries []string
}

func (p *TempoPoller) Snapshot(ctx context.Context) (ctxmodel.Snapshot, error) {
	if p.Router == nil {
		return ctxmodel.Snapshot{}, errors.New("router required")
	}
	queries := p.Queries
	if len(queries) == 0 {
		queries = []string{"{ }"}
	}
	var snap ctxmodel.Snapshot
	for _, query := range queries {
		_, err := p.Router.Execute(ctx, tools.ExecuteRequest{Tool: "tempo", Action: "traceql", Input: map[string]any{"query": query}, Context: p.Context})
		if err != nil {
			continue
		}
		id := nodeID("tempo", "query", hashTempo(query))
		labels := p.withLabels(map[string]string{"query": query})
		snap.Nodes = append(snap.Nodes, node("tempo.query", id, query, labels))
	}
	return snap, nil
}

func hashTempo(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:8])
}
