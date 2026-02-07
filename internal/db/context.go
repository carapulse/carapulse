package db

import (
	"context"
	"encoding/json"
	"errors"

	ctxmodel "carapulse/internal/context"
)

var errInvalidContextNode = errors.New("invalid context node")
var errInvalidContextEdge = errors.New("invalid context edge")
var errInvalidContextService = errors.New("invalid context service")

func (d *DB) UpsertContextNode(ctx context.Context, node ctxmodel.Node) error {
	if node.NodeID == "" || node.Kind == "" || node.Name == "" {
		return errInvalidContextNode
	}
	labelsJSON := []byte("null")
	if node.Labels != nil {
		labelsJSON, _ = json.Marshal(node.Labels)
	}
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO context_nodes(node_id, kind, name, labels_json, owner_team)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (node_id) DO UPDATE SET
			kind=EXCLUDED.kind,
			name=EXCLUDED.name,
			labels_json=EXCLUDED.labels_json,
			owner_team=EXCLUDED.owner_team
	`, node.NodeID, node.Kind, node.Name, labelsJSON, nullString(node.OwnerTeam))
	return err
}

func (d *DB) UpsertContextEdge(ctx context.Context, edge ctxmodel.Edge) error {
	if edge.FromNodeID == "" || edge.ToNodeID == "" || edge.Relation == "" {
		return errInvalidContextEdge
	}
	edgeID := edge.EdgeID
	if edgeID == "" {
		edgeID = newID("edge")
	}
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO context_edges(edge_id, from_node_id, to_node_id, relation)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (edge_id) DO UPDATE SET
			from_node_id=EXCLUDED.from_node_id,
			to_node_id=EXCLUDED.to_node_id,
			relation=EXCLUDED.relation
	`, edgeID, edge.FromNodeID, edge.ToNodeID, edge.Relation)
	return err
}

func (d *DB) GetServiceGraph(ctx context.Context, service string) ([]byte, error) {
	if service == "" {
		return nil, errInvalidContextService
	}
	query := `
		WITH service_nodes AS (
			SELECT node_id FROM context_nodes WHERE kind='service' AND (node_id=$1 OR name=$1)
		),
		edges AS (
			SELECT edge_id, from_node_id, to_node_id, relation
			FROM context_edges
			WHERE from_node_id IN (SELECT node_id FROM service_nodes)
			   OR to_node_id IN (SELECT node_id FROM service_nodes)
		),
		nodes AS (
			SELECT node_id, kind, name, labels_json, owner_team
			FROM context_nodes
			WHERE node_id IN (SELECT node_id FROM service_nodes)
			   OR node_id IN (SELECT from_node_id FROM edges)
			   OR node_id IN (SELECT to_node_id FROM edges)
		)
		SELECT jsonb_build_object(
			'nodes', COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'node_id', node_id,
					'kind', kind,
					'name', name,
					'labels', labels_json,
					'owner_team', owner_team
				) ORDER BY name) FROM nodes
			), '[]'::jsonb),
			'edges', COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'edge_id', edge_id,
					'from_node_id', from_node_id,
					'to_node_id', to_node_id,
					'relation', relation
				) ORDER BY edge_id) FROM edges
			), '[]'::jsonb)
		)`
	row := d.conn.QueryRowContext(ctx, query, service)
	var out []byte
	if err := row.Scan(&out); err != nil {
		return nil, err
	}
	return out, nil
}
