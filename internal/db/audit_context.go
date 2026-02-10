package db

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type AuditFilter struct {
	From     time.Time
	To       time.Time
	ActorID  string
	Action   string
	Decision string
	Limit    int
	Offset   int
}

func (d *DB) ListAuditEvents(ctx context.Context, filter AuditFilter) ([]byte, int, error) {
	limit, offset := clampPagination(filter.Limit, filter.Offset)
	where := []string{}
	args := []any{}
	arg := 1
	if !filter.From.IsZero() {
		where = append(where, fmt.Sprintf("occurred_at >= $%d", arg))
		args = append(args, filter.From)
		arg++
	}
	if !filter.To.IsZero() {
		where = append(where, fmt.Sprintf("occurred_at <= $%d", arg))
		args = append(args, filter.To)
		arg++
	}
	if filter.ActorID != "" {
		where = append(where, fmt.Sprintf("actor_json->>'id' = $%d", arg))
		args = append(args, filter.ActorID)
		arg++
	}
	if filter.Action != "" {
		where = append(where, fmt.Sprintf("action = $%d", arg))
		args = append(args, filter.Action)
		arg++
	}
	if filter.Decision != "" {
		where = append(where, fmt.Sprintf("decision = $%d", arg))
		args = append(args, filter.Decision)
		arg++
	}
	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}
	query := fmt.Sprintf(`WITH filtered AS (SELECT *, COUNT(*) OVER() AS total_count FROM audit_events%s),
	paged AS (SELECT * FROM filtered ORDER BY occurred_at DESC LIMIT $%d OFFSET $%d)
	SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			'event_id', event_id,
			'occurred_at', occurred_at,
			'actor', actor_json,
			'action', action,
			'decision', decision,
			'context', context_json,
			'evidence_refs', evidence_refs_json,
			'hash', hash
		) ORDER BY occurred_at DESC
	), '[]'::jsonb),
	COALESCE((SELECT total_count FROM paged LIMIT 1), 0)
	FROM paged`, whereClause, arg, arg+1)
	args = append(args, limit, offset)
	row := d.conn.QueryRowContext(ctx, query, args...)
	var out []byte
	var total int
	if err := row.Scan(&out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (d *DB) ListContextServices(ctx context.Context, limit, offset int) ([]byte, int, error) {
	limit, offset = clampPagination(limit, offset)
	query := `WITH total AS (SELECT COUNT(*) AS cnt FROM context_nodes WHERE kind = 'service')
	SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			'service_id', node_id,
			'name', name,
			'owner_team', owner_team,
			'labels', labels_json
		) ORDER BY name
	), '[]'::jsonb),
	(SELECT cnt FROM total)
	FROM (SELECT * FROM context_nodes WHERE kind = 'service' ORDER BY name LIMIT $1 OFFSET $2) AS sub`
	row := d.conn.QueryRowContext(ctx, query, limit, offset)
	var out []byte
	var total int
	if err := row.Scan(&out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
