package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type operatorMemoryPayload struct {
	TenantID string          `json:"tenant_id"`
	Title    string          `json:"title"`
	Body     string          `json:"body"`
	Tags     json.RawMessage `json:"tags"`
	Metadata json.RawMessage `json:"metadata"`
	OwnerID  string          `json:"owner_actor_id"`
}

func (d *DB) CreateOperatorMemory(ctx context.Context, payload []byte) (string, error) {
	if d == nil || d.conn == nil {
		return "", errors.New("db required")
	}
	var data operatorMemoryPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", err
	}
	tenant := strings.TrimSpace(data.TenantID)
	title := strings.TrimSpace(data.Title)
	body := strings.TrimSpace(data.Body)
	if tenant == "" || title == "" || body == "" {
		return "", errors.New("tenant_id, title, body required")
	}
	id := newID("memory")
	now := time.Now().UTC()
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO operator_memory(memory_id, tenant_id, title, body, tags, metadata, owner_actor_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, id, tenant, title, body, nullJSON(data.Tags), nullJSON(data.Metadata), nullString(strings.TrimSpace(data.OwnerID)), now, now)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *DB) ListOperatorMemory(ctx context.Context, tenantID string) ([]byte, error) {
	if d == nil || d.conn == nil {
		return nil, errors.New("db required")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, errors.New("tenant_id required")
	}
	row := d.conn.QueryRowContext(ctx, `
		SELECT COALESCE(jsonb_agg(
			jsonb_build_object(
				'memory_id', memory_id,
				'tenant_id', tenant_id,
				'title', title,
				'body', body,
				'tags', tags,
				'metadata', metadata,
				'owner_actor_id', owner_actor_id,
				'created_at', created_at,
				'updated_at', updated_at
			) ORDER BY created_at DESC
		), '[]'::jsonb)
		FROM operator_memory WHERE tenant_id=$1
	`, tenantID)
	var out []byte
	if err := row.Scan(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) GetOperatorMemory(ctx context.Context, memoryID string) ([]byte, error) {
	if d == nil || d.conn == nil {
		return nil, errors.New("db required")
	}
	memoryID = strings.TrimSpace(memoryID)
	if memoryID == "" {
		return nil, errors.New("memory_id required")
	}
	row := d.conn.QueryRowContext(ctx, `
		SELECT tenant_id, title, body, tags, metadata, owner_actor_id, created_at, updated_at
		FROM operator_memory WHERE memory_id=$1
	`, memoryID)
	var tenantID, title, body, owner string
	var tags, metadata []byte
	var createdAt, updatedAt time.Time
	if err := row.Scan(&tenantID, &title, &body, &tags, &metadata, &owner, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	out := map[string]any{
		"memory_id":      memoryID,
		"tenant_id":      tenantID,
		"title":          title,
		"body":           body,
		"tags":           json.RawMessage(defaultJSON(tags)),
		"metadata":       json.RawMessage(defaultJSON(metadata)),
		"owner_actor_id": owner,
		"created_at":     createdAt.UTC().Format(time.RFC3339),
		"updated_at":     updatedAt.UTC().Format(time.RFC3339),
	}
	return json.Marshal(out)
}

func (d *DB) UpdateOperatorMemory(ctx context.Context, memoryID string, payload []byte) error {
	if d == nil || d.conn == nil {
		return errors.New("db required")
	}
	memoryID = strings.TrimSpace(memoryID)
	if memoryID == "" {
		return errors.New("memory_id required")
	}
	var data operatorMemoryPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}
	tenant := strings.TrimSpace(data.TenantID)
	title := strings.TrimSpace(data.Title)
	body := strings.TrimSpace(data.Body)
	if tenant == "" || title == "" || body == "" {
		return errors.New("tenant_id, title, body required")
	}
	_, err := d.conn.ExecContext(ctx, `
		UPDATE operator_memory
		SET tenant_id=$2, title=$3, body=$4, tags=$5, metadata=$6, owner_actor_id=$7, updated_at=$8
		WHERE memory_id=$1
	`, memoryID, tenant, title, body, nullJSON(data.Tags), nullJSON(data.Metadata), nullString(strings.TrimSpace(data.OwnerID)), time.Now().UTC())
	return err
}

func (d *DB) DeleteOperatorMemory(ctx context.Context, memoryID string) error {
	if d == nil || d.conn == nil {
		return errors.New("db required")
	}
	memoryID = strings.TrimSpace(memoryID)
	if memoryID == "" {
		return errors.New("memory_id required")
	}
	_, err := d.conn.ExecContext(ctx, `DELETE FROM operator_memory WHERE memory_id=$1`, memoryID)
	return err
}
