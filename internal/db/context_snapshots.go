package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func (d *DB) InsertContextSnapshot(ctx context.Context, source string, nodesJSON, edgesJSON, labelsJSON []byte) (string, error) {
	if d == nil || d.conn == nil {
		return "", errors.New("db required")
	}
	if len(nodesJSON) == 0 || len(edgesJSON) == 0 {
		return "", errors.New("snapshot data required")
	}
	id := newID("snapshot")
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO context_snapshots(snapshot_id, source, collected_at, nodes_json, edges_json, labels_json)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, source, time.Now().UTC(), json.RawMessage(nodesJSON), json.RawMessage(edgesJSON), nullJSON(labelsJSON))
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *DB) ListContextSnapshots(ctx context.Context, limit, offset int) ([]byte, int, error) {
	limit, offset = clampPagination(limit, offset)
	query := `WITH total AS (SELECT COUNT(*) AS cnt FROM context_snapshots)
	SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			'snapshot_id', snapshot_id,
			'source', source,
			'collected_at', collected_at,
			'labels', labels_json
		) ORDER BY collected_at DESC
	), '[]'::jsonb),
	(SELECT cnt FROM total)
	FROM (SELECT * FROM context_snapshots ORDER BY collected_at DESC LIMIT $1 OFFSET $2) AS sub`
	row := d.conn.QueryRowContext(ctx, query, limit, offset)
	var out []byte
	var total int
	if err := row.Scan(&out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (d *DB) GetContextSnapshot(ctx context.Context, snapshotID string) ([]byte, error) {
	if d == nil || d.conn == nil {
		return nil, errors.New("db required")
	}
	snapshotID = strings.TrimSpace(snapshotID)
	if snapshotID == "" {
		return nil, errors.New("snapshot_id required")
	}
	row := d.conn.QueryRowContext(ctx, `
		SELECT snapshot_id, source, collected_at, nodes_json, edges_json, labels_json
		FROM context_snapshots WHERE snapshot_id=$1
	`, snapshotID)
	var source string
	var collectedAt time.Time
	var nodes, edges, labels []byte
	if err := row.Scan(&snapshotID, &source, &collectedAt, &nodes, &edges, &labels); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	out := map[string]any{
		"snapshot_id":  snapshotID,
		"source":       source,
		"collected_at": collectedAt,
		"nodes":        json.RawMessage(nodes),
		"edges":        json.RawMessage(edges),
		"labels":       json.RawMessage(defaultJSON(labels)),
	}
	return json.Marshal(out)
}
