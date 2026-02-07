package collectors

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/tools"
)

type AlertmanagerPoller struct {
	Base
}

func (p *AlertmanagerPoller) Snapshot(ctx context.Context) (ctxmodel.Snapshot, error) {
	if p.Router == nil {
		return ctxmodel.Snapshot{}, errors.New("router required")
	}
	resp, err := p.Router.Execute(ctx, tools.ExecuteRequest{Tool: "alertmanager", Action: "alerts_list", Input: map[string]any{}, Context: p.Context})
	if err != nil {
		return ctxmodel.Snapshot{}, err
	}
	var alerts []map[string]any
	if err := json.Unmarshal(resp.Output, &alerts); err != nil {
		return ctxmodel.Snapshot{}, err
	}
	var snap ctxmodel.Snapshot
	for _, alert := range alerts {
		fp, _ := alert["fingerprint"].(string)
		fp = strings.TrimSpace(fp)
		if fp == "" {
			continue
		}
		labels := p.withLabels(map[string]string{})
		if l, ok := alert["labels"].(map[string]any); ok {
			for k, v := range l {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					labels[k] = s
				}
			}
		}
		alertID := nodeID("alert", fp)
		snap.Nodes = append(snap.Nodes, node("alert", alertID, fp, labels))
		service := labels["service"]
		if service == "" {
			service = labels["app"]
		}
		if strings.TrimSpace(service) != "" {
			svcID := nodeID("service", service)
			snap.Nodes = append(snap.Nodes, node("service", svcID, service, labels))
			snap.Edges = append(snap.Edges, edge(nodeID("edge", alertID, svcID), alertID, svcID, "alerts"))
		}
	}
	return snap, nil
}
