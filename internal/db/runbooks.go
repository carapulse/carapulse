package db

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type runbookPayload struct {
	TenantID string          `json:"tenant_id"`
	Service  string          `json:"service"`
	Name     string          `json:"name"`
	Version  int             `json:"version"`
	Tags     json.RawMessage `json:"tags"`
	Body     string          `json:"body"`
	Spec     json.RawMessage `json:"spec"`
}

func (d *DB) CreateRunbook(ctx context.Context, payload []byte) (string, error) {
	if d == nil || d.conn == nil {
		return "", errors.New("db required")
	}
	var data runbookPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", err
	}
	tenant := strings.TrimSpace(data.TenantID)
	service := strings.TrimSpace(data.Service)
	name := strings.TrimSpace(data.Name)
	if tenant == "" || service == "" || name == "" {
		return "", errors.New("tenant_id, service and name required")
	}
	version := data.Version
	if version <= 0 {
		row := d.conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM runbooks WHERE tenant_id=$1 AND service=$2 AND name=$3`, tenant, service, name)
		var max int
		if err := row.Scan(&max); err != nil {
			return "", err
		}
		version = max + 1
	}
	id := newID("runbook")
	var tagsJSON any
	if len(data.Tags) > 0 {
		tagsJSON = data.Tags
	}
	var specJSON any
	if len(data.Spec) > 0 {
		specJSON = data.Spec
	}
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO runbooks(runbook_id, tenant_id, service, name, version, tags, body, spec_json, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, id, tenant, service, name, version, tagsJSON, nullString(strings.TrimSpace(data.Body)), specJSON, time.Now().UTC())
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *DB) ListRunbooks(ctx context.Context) ([]byte, error) {
	query := `SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			"runbook_id", runbook_id,
			"tenant_id", tenant_id,
			"service", service,
			"name", name,
			"version", version,
			"tags", tags,
			"created_at", created_at
		) ORDER BY created_at DESC
	), '[]'::jsonb) FROM runbooks`
	row := d.conn.QueryRowContext(ctx, query)
	var out []byte
	if err := row.Scan(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) GetRunbook(ctx context.Context, runbookID string) ([]byte, error) {
	runbookID = strings.TrimSpace(runbookID)
	if runbookID == "" {
		return nil, errors.New("runbook_id required")
	}
	row := d.conn.QueryRowContext(ctx, `
		SELECT runbook_id, tenant_id, service, name, version, tags, body, spec_json, created_at
		FROM runbooks WHERE runbook_id=$1
	`, runbookID)
	var tenantID, service, name, body string
	var version int
	var tags, spec []byte
	var createdAt time.Time
	if err := row.Scan(&runbookID, &tenantID, &service, &name, &version, &tags, &body, &spec, &createdAt); err != nil {
		return nil, err
	}
	out := map[string]any{
		"runbook_id": runbookID,
		"tenant_id":  tenantID,
		"service":    service,
		"name":       name,
		"version":    version,
		"tags":       json.RawMessage(defaultJSON(tags)),
		"body":       body,
		"spec":       json.RawMessage(defaultJSON(spec)),
		"created_at": createdAt.UTC().Format(time.RFC3339),
	}
	return json.Marshal(out)
}

func defaultJSON(data []byte) []byte {
	if len(data) == 0 {
		return []byte("null")
	}
	return data
}
