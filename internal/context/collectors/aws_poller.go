package collectors

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/tools"
)

type AWSPoller struct {
	Base
	TagFilters []map[string]any
	Region     string
}

func (p *AWSPoller) Snapshot(ctx context.Context) (ctxmodel.Snapshot, error) {
	if p.Router == nil {
		return ctxmodel.Snapshot{}, errors.New("router required")
	}
	input := map[string]any{}
	if len(p.TagFilters) > 0 {
		input["tag_filters"] = p.TagFilters
	}
	if strings.TrimSpace(p.Region) != "" {
		input["region"] = p.Region
	}
	resp, err := p.Router.Execute(ctx, tools.ExecuteRequest{Tool: "aws", Action: "tagging-get-resources", Input: input, Context: p.Context})
	if err != nil {
		return ctxmodel.Snapshot{}, err
	}
	return snapshotFromAWSResources(resp.Output, p.withLabels(nil)), nil
}

func snapshotFromAWSResources(payload []byte, labels map[string]string) ctxmodel.Snapshot {
	if len(payload) == 0 {
		return ctxmodel.Snapshot{}
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return ctxmodel.Snapshot{}
	}
	items, _ := raw["ResourceTagMappingList"].([]any)
	var snap ctxmodel.Snapshot
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		arn, _ := m["ResourceARN"].(string)
		if arn == "" {
			continue
		}
		service, account := parseARN(arn)
		resID := nodeID("aws", service, arn)
		resLabels := map[string]string{"service": service, "account": account}
		for k, v := range labels {
			resLabels[k] = v
		}
		snap.Nodes = append(snap.Nodes, node("aws.resource", resID, arn, resLabels))
		if account != "" {
			acctID := nodeID("aws", "account", account)
			snap.Nodes = append(snap.Nodes, node("aws.account", acctID, account, labels))
			snap.Edges = append(snap.Edges, edge(nodeID("edge", acctID, resID), acctID, resID, "contains"))
		}
	}
	return snap
}

func parseARN(arn string) (string, string) {
	parts := strings.Split(arn, ":")
	if len(parts) < 6 {
		return "", ""
	}
	return parts[2], parts[4]
}
