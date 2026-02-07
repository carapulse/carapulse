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
}

func (d *DB) ListAuditEvents(ctx context.Context, filter AuditFilter) ([]byte, error) {
	query := `SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			"event_id", event_id,
			"occurred_at", occurred_at,
			"actor", actor_json,
			"action", action,
			"decision", decision,
			"context", context_json,
			"evidence_refs", evidence_refs_json,
			"hash", hash
		) ORDER BY occurred_at DESC
	), '[]'::jsonb) FROM audit_events`
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
	if len(where) > 0 {
		query = query + " WHERE " + strings.Join(where, " AND ")
	}
	row := d.conn.QueryRowContext(ctx, query, args...)
	var out []byte
	if err := row.Scan(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) ListContextServices(ctx context.Context) ([]byte, error) {
	query := `SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			"service_id", node_id,
			"name", name,
			"owner_team", owner_team,
			"labels", labels_json
		) ORDER BY name
	), '[]'::jsonb) FROM context_nodes WHERE kind = 'service'`
	row := d.conn.QueryRowContext(ctx, query)
	var out []byte
	if err := row.Scan(&out); err != nil {
		return nil, err
	}
	return out, nil
}
