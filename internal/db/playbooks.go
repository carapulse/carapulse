package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type playbookPayload struct {
	TenantID string          `json:"tenant_id"`
	Name     string          `json:"name"`
	Version  int             `json:"version"`
	Tags     json.RawMessage `json:"tags"`
	Spec     json.RawMessage `json:"spec"`
}

func (d *DB) CreatePlaybook(ctx context.Context, payload []byte) (string, error) {
	if len(payload) == 0 {
		return "", errors.New("payload required")
	}
	var data playbookPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", err
	}
	tenant := strings.TrimSpace(data.TenantID)
	if tenant == "" || data.Name == "" {
		return "", errors.New("tenant_id and name required")
	}
	if data.Version == 0 {
		data.Version = 1
	}
	if len(data.Spec) == 0 {
		return "", errors.New("spec required")
	}
	id := newID("playbook")
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO playbooks(playbook_id, tenant_id, name, version, tags, spec_json, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, id, tenant, data.Name, data.Version, nullJSON(data.Tags), data.Spec, time.Now().UTC())
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *DB) ListPlaybooks(ctx context.Context, limit, offset int) ([]byte, int, error) {
	limit, offset = clampPagination(limit, offset)
	query := `WITH total AS (SELECT COUNT(*) AS cnt FROM playbooks)
	SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			'playbook_id', playbook_id,
			'tenant_id', tenant_id,
			'name', name,
			'version', version,
			'tags', tags,
			'spec', spec_json,
			'created_at', created_at
		) ORDER BY created_at DESC
	), '[]'::jsonb),
	(SELECT cnt FROM total)
	FROM (SELECT * FROM playbooks ORDER BY created_at DESC LIMIT $1 OFFSET $2) AS sub`
	row := d.conn.QueryRowContext(ctx, query, limit, offset)
	var out []byte
	var total int
	if err := row.Scan(&out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (d *DB) GetPlaybook(ctx context.Context, playbookID string) ([]byte, error) {
	row := d.conn.QueryRowContext(ctx, `
		SELECT tenant_id, name, version, tags, spec_json, created_at
		FROM playbooks WHERE playbook_id=$1
	`, playbookID)
	var tenantID, name string
	var version int
	var tags []byte
	var spec []byte
	var createdAt time.Time
	if err := row.Scan(&tenantID, &name, &version, &tags, &spec, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	out := map[string]any{
		"playbook_id": playbookID,
		"tenant_id":   tenantID,
		"name":        name,
		"version":     version,
		"tags":        json.RawMessage(tags),
		"spec":        json.RawMessage(spec),
		"created_at":  createdAt,
	}
	return json.Marshal(out)
}

func (d *DB) DeletePlaybook(ctx context.Context, playbookID string) error {
	if d == nil || d.conn == nil {
		return errors.New("db not available")
	}
	_, err := d.conn.ExecContext(ctx, "DELETE FROM playbooks WHERE playbook_id=$1", playbookID)
	return err
}

func nullJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
