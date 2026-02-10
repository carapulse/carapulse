package db

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type workflowPayload struct {
	TenantID string          `json:"tenant_id"`
	Name     string          `json:"name"`
	Version  int             `json:"version"`
	Spec     json.RawMessage `json:"spec"`
}

func (d *DB) CreateWorkflowCatalog(ctx context.Context, payload []byte) (string, error) {
	if d == nil || d.conn == nil {
		return "", errors.New("db required")
	}
	var data workflowPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", err
	}
	tenant := strings.TrimSpace(data.TenantID)
	name := strings.TrimSpace(data.Name)
	if name == "" {
		return "", errors.New("name required")
	}
	if data.Version <= 0 {
		data.Version = 1
	}
	if len(data.Spec) == 0 {
		return "", errors.New("spec required")
	}
	id := newID("workflow")
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO workflow_catalog(workflow_id, tenant_id, name, version, spec_json, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, nullString(tenant), name, data.Version, data.Spec, time.Now().UTC())
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *DB) ListWorkflowCatalog(ctx context.Context, limit, offset int) ([]byte, int, error) {
	limit, offset = clampPagination(limit, offset)
	query := `WITH total AS (SELECT COUNT(*) AS cnt FROM workflow_catalog)
	SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			'workflow_id', workflow_id,
			'tenant_id', tenant_id,
			'name', name,
			'version', version,
			'spec', spec_json,
			'created_at', created_at
		) ORDER BY created_at DESC
	), '[]'::jsonb),
	(SELECT cnt FROM total)
	FROM (SELECT * FROM workflow_catalog ORDER BY created_at DESC LIMIT $1 OFFSET $2) AS sub`
	row := d.conn.QueryRowContext(ctx, query, limit, offset)
	var out []byte
	var total int
	if err := row.Scan(&out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
