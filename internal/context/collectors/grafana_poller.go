package collectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/tools"
)

type GrafanaPoller struct {
	Base
	Query     string
	FolderIDs []int
}

func (p *GrafanaPoller) Snapshot(ctx context.Context) (ctxmodel.Snapshot, error) {
	if p.Router == nil {
		return ctxmodel.Snapshot{}, errors.New("router required")
	}
	input := map[string]any{}
	if strings.TrimSpace(p.Query) != "" {
		input["query"] = p.Query
	}
	if len(p.FolderIDs) > 0 {
		var ids []any
		for _, id := range p.FolderIDs {
			if id > 0 {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			input["folder_ids"] = ids
		}
	}
	resp, err := p.Router.Execute(ctx, tools.ExecuteRequest{Tool: "grafana", Action: "dashboard_list", Input: input, Context: p.Context})
	if err != nil {
		return ctxmodel.Snapshot{}, err
	}
	return snapshotFromGrafana(resp.Output, p.withLabels(nil)), nil
}

func snapshotFromGrafana(payload []byte, labels map[string]string) ctxmodel.Snapshot {
	if len(payload) == 0 {
		return ctxmodel.Snapshot{}
	}
	var items []map[string]any
	if err := json.Unmarshal(payload, &items); err != nil {
		return ctxmodel.Snapshot{}
	}
	var snap ctxmodel.Snapshot
	for _, item := range items {
		uid, _ := item["uid"].(string)
		title, _ := item["title"].(string)
		if strings.TrimSpace(uid) == "" {
			continue
		}
		id := nodeID("grafana", "dashboard", uid)
		snap.Nodes = append(snap.Nodes, node("grafana.dashboard", id, title, labels))
		if folderID, ok := item["folderId"].(float64); ok && folderID > 0 {
			folderNode := node("grafana.folder", nodeID("grafana", "folder", fmt.Sprintf("%d", int(folderID))), fmt.Sprintf("%d", int(folderID)), labels)
			snap.Nodes = append(snap.Nodes, folderNode)
			snap.Edges = append(snap.Edges, edge(nodeID("edge", folderNode.NodeID, id), folderNode.NodeID, id, "contains"))
		}
	}
	return snap
}
