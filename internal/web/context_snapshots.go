package web

import (
	"encoding/json"
	"net/http"
	"strings"

	ctxmodel "carapulse/internal/context"
)

type snapshotPayload struct {
	SnapshotID string
	Labels     map[string]any
	Nodes      []ctxmodel.Node
	Edges      []ctxmodel.Edge
}

func (s *Server) handleContextSnapshotByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
	if tenantID == "" {
		http.Error(w, "tenant_id required", http.StatusBadRequest)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/context/snapshots/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "snapshot_id required", http.StatusBadRequest)
		return
	}
	snapshotID := parts[0]
	if len(parts) == 1 {
		if err := policyCheckTenantRead(s, r, "context.snapshot.get", tenantID); err != nil {
			s.auditEvent(r.Context(), "context.snapshot.get", "deny", map[string]any{"snapshot_id": snapshotID}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload, err := s.DB.GetContextSnapshot(r.Context(), snapshotID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if payload == nil {
			http.NotFound(w, r)
			return
		}
		item, err := parseSnapshotPayload(payload)
		if err != nil {
			http.Error(w, "decode error", http.StatusInternalServerError)
			return
		}
		if !tenantMatch(tenantFromLabels(map[string]any{"labels": item.Labels}), tenantID, false) {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(payload)
		return
	}
	if len(parts) == 2 && parts[1] == "diff" {
		if err := policyCheckTenantRead(s, r, "context.snapshot.diff", tenantID); err != nil {
			s.auditEvent(r.Context(), "context.snapshot.diff", "deny", map[string]any{"snapshot_id": snapshotID}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		baseID := strings.TrimSpace(r.URL.Query().Get("base"))
		if baseID == "" {
			http.Error(w, "base snapshot required", http.StatusBadRequest)
			return
		}
		payloadA, err := s.DB.GetContextSnapshot(r.Context(), snapshotID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		payloadB, err := s.DB.GetContextSnapshot(r.Context(), baseID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if payloadA == nil || payloadB == nil {
			http.NotFound(w, r)
			return
		}
		a, err := parseSnapshotPayload(payloadA)
		if err != nil {
			http.Error(w, "decode error", http.StatusInternalServerError)
			return
		}
		b, err := parseSnapshotPayload(payloadB)
		if err != nil {
			http.Error(w, "decode error", http.StatusInternalServerError)
			return
		}
		if !tenantMatch(tenantFromLabels(map[string]any{"labels": a.Labels}), tenantID, false) ||
			!tenantMatch(tenantFromLabels(map[string]any{"labels": b.Labels}), tenantID, false) {
			http.NotFound(w, r)
			return
		}
		out := diffSnapshots(a, b)
		out["snapshot_id"] = snapshotID
		out["base_snapshot_id"] = baseID
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	http.Error(w, "invalid snapshot path", http.StatusBadRequest)
}

func parseSnapshotPayload(payload []byte) (snapshotPayload, error) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return snapshotPayload{}, err
	}
	item := snapshotPayload{}
	if id, ok := raw["snapshot_id"].(string); ok {
		item.SnapshotID = id
	}
	if labels, ok := raw["labels"].(map[string]any); ok {
		item.Labels = labels
	}
	if rawNodes, ok := raw["nodes"]; ok {
		if data, err := json.Marshal(rawNodes); err == nil {
			_ = json.Unmarshal(data, &item.Nodes)
		}
	}
	if rawEdges, ok := raw["edges"]; ok {
		if data, err := json.Marshal(rawEdges); err == nil {
			_ = json.Unmarshal(data, &item.Edges)
		}
	}
	return item, nil
}

func diffSnapshots(current snapshotPayload, base snapshotPayload) map[string]any {
	addedNodes, removedNodes := diffNodes(current.Nodes, base.Nodes)
	addedEdges, removedEdges := diffEdges(current.Edges, base.Edges)
	return map[string]any{
		"added_nodes":   addedNodes,
		"removed_nodes": removedNodes,
		"added_edges":   addedEdges,
		"removed_edges": removedEdges,
	}
}

func diffNodes(current, base []ctxmodel.Node) ([]ctxmodel.Node, []ctxmodel.Node) {
	baseMap := map[string]ctxmodel.Node{}
	for _, node := range base {
		if node.NodeID != "" {
			baseMap[node.NodeID] = node
		}
	}
	currMap := map[string]ctxmodel.Node{}
	for _, node := range current {
		if node.NodeID != "" {
			currMap[node.NodeID] = node
		}
	}
	var added []ctxmodel.Node
	var removed []ctxmodel.Node
	for id, node := range currMap {
		if _, ok := baseMap[id]; !ok {
			added = append(added, node)
		}
	}
	for id, node := range baseMap {
		if _, ok := currMap[id]; !ok {
			removed = append(removed, node)
		}
	}
	return added, removed
}

func diffEdges(current, base []ctxmodel.Edge) ([]ctxmodel.Edge, []ctxmodel.Edge) {
	baseMap := map[string]ctxmodel.Edge{}
	for _, edge := range base {
		if edge.EdgeID != "" {
			baseMap[edge.EdgeID] = edge
		}
	}
	currMap := map[string]ctxmodel.Edge{}
	for _, edge := range current {
		if edge.EdgeID != "" {
			currMap[edge.EdgeID] = edge
		}
	}
	var added []ctxmodel.Edge
	var removed []ctxmodel.Edge
	for id, edge := range currMap {
		if _, ok := baseMap[id]; !ok {
			added = append(added, edge)
		}
	}
	for id, edge := range baseMap {
		if _, ok := currMap[id]; !ok {
			removed = append(removed, edge)
		}
	}
	return added, removed
}
