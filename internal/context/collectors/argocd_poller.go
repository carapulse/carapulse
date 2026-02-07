package collectors

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/tools"
)

type ArgoCDPoller struct {
	Base
	Apps []string
}

func (p *ArgoCDPoller) Snapshot(ctx context.Context) (ctxmodel.Snapshot, error) {
	if p.Router == nil {
		return ctxmodel.Snapshot{}, errors.New("router required")
	}
	resp, err := p.Router.Execute(ctx, tools.ExecuteRequest{Tool: "argocd", Action: "list", Input: map[string]any{}, Context: p.Context})
	if err != nil {
		return ctxmodel.Snapshot{}, err
	}
	return snapshotFromArgoList(resp.Output, p.withLabels(nil), p.Apps), nil
}

func snapshotFromArgoList(payload []byte, labels map[string]string, allow []string) ctxmodel.Snapshot {
	if len(payload) == 0 {
		return ctxmodel.Snapshot{}
	}
	var apps []map[string]any
	if err := json.Unmarshal(payload, &apps); err != nil {
		return ctxmodel.Snapshot{}
	}
	var snap ctxmodel.Snapshot
	allowed := map[string]struct{}{}
	for _, name := range allow {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	for _, app := range apps {
		name := stringField(app, "metadata", "name")
		if name == "" {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[name]; !ok {
				continue
			}
		}
		appID := nodeID("argocd", "app", name)
		snap.Nodes = append(snap.Nodes, node("argocd.app", appID, name, labels))
		namespace := stringField(app, "spec", "destination", "namespace")
		if namespace != "" {
			nsID := nodeID("k8s", "namespace", namespace)
			snap.Nodes = append(snap.Nodes, node("k8s.namespace", nsID, namespace, labels))
			snap.Edges = append(snap.Edges, edge(nodeID("edge", appID, nsID), appID, nsID, "targets"))
		}
	}
	return snap
}

func stringField(obj map[string]any, path ...string) string {
	var cur any = obj
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur, ok = m[key]
		if !ok {
			return ""
		}
	}
	if s, ok := cur.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}
