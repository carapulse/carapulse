package collectors

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/tools"
)

type HelmPoller struct {
	Base
	Namespaces []string
}

func (p *HelmPoller) Snapshot(ctx context.Context) (ctxmodel.Snapshot, error) {
	if p.Router == nil {
		return ctxmodel.Snapshot{}, errors.New("router required")
	}
	namespaces := p.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{""}
	}
	var snap ctxmodel.Snapshot
	for _, ns := range namespaces {
		input := map[string]any{}
		if strings.TrimSpace(ns) != "" {
			input["namespace"] = ns
		}
		resp, err := p.Router.Execute(ctx, tools.ExecuteRequest{Tool: "helm", Action: "list", Input: input, Context: p.Context})
		if err != nil {
			return ctxmodel.Snapshot{}, err
		}
		part := snapshotFromHelmList(resp.Output, p.withLabels(nil))
		snap.Nodes = append(snap.Nodes, part.Nodes...)
		snap.Edges = append(snap.Edges, part.Edges...)
	}
	return snap, nil
}

func snapshotFromHelmList(payload []byte, labels map[string]string) ctxmodel.Snapshot {
	if len(payload) == 0 {
		return ctxmodel.Snapshot{}
	}
	var releases []map[string]any
	if err := json.Unmarshal(payload, &releases); err != nil {
		return ctxmodel.Snapshot{}
	}
	var snap ctxmodel.Snapshot
	for _, rel := range releases {
		name, _ := rel["name"].(string)
		namespace, _ := rel["namespace"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		id := nodeID("helm", "release", namespace, name)
		relLabels := map[string]string{"namespace": namespace}
		for k, v := range labels {
			relLabels[k] = v
		}
		snap.Nodes = append(snap.Nodes, node("helm.release", id, name, relLabels))
		if namespace != "" {
			nsID := nodeID("k8s", "namespace", namespace)
			snap.Nodes = append(snap.Nodes, node("k8s.namespace", nsID, namespace, labels))
			snap.Edges = append(snap.Edges, edge(nodeID("edge", nsID, id), nsID, id, "contains"))
		}
	}
	return snap
}
