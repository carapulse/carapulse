package collectors

import (
	"context"
	"strings"
	"time"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/tools"
)

type ToolRunner interface {
	Execute(ctx context.Context, req tools.ExecuteRequest) (tools.ExecuteResponse, error)
}

type Config struct {
	Interval time.Duration
	Enabled  bool
}

type Base struct {
	Router  ToolRunner
	Context tools.ContextRef
	Labels  map[string]string
}

func (b Base) withLabels(extra map[string]string) map[string]string {
	labels := map[string]string{}
	for k, v := range b.Labels {
		labels[k] = v
	}
	for k, v := range extra {
		labels[k] = v
	}
	return labels
}

func nodeID(parts ...string) string {
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, "/")
}

func node(kind, id, name string, labels map[string]string) ctxmodel.Node {
	return ctxmodel.Node{
		NodeID: id,
		Kind:   kind,
		Name:   name,
		Labels: labels,
	}
}

func edge(id, from, to, relation string) ctxmodel.Edge {
	return ctxmodel.Edge{
		EdgeID:     id,
		FromNodeID: from,
		ToNodeID:   to,
		Relation:   relation,
	}
}
