package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type sessionPayload struct {
	Name     string          `json:"name"`
	TenantID string          `json:"tenant_id"`
	GroupID  string          `json:"group_id"`
	OwnerID  string          `json:"owner_actor_id"`
	Metadata json.RawMessage `json:"metadata"`
}

type sessionMemberPayload struct {
	MemberID string `json:"member_id"`
	Role     string `json:"role"`
}

func (d *DB) CreateSession(ctx context.Context, payload []byte) (string, error) {
	if d == nil || d.conn == nil {
		return "", errors.New("db required")
	}
	var data sessionPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", err
	}
	name := strings.TrimSpace(data.Name)
	tenant := strings.TrimSpace(data.TenantID)
	if name == "" || tenant == "" {
		return "", errors.New("name and tenant_id required")
	}
	id := newID("session")
	now := time.Now().UTC()
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO sessions(session_id, name, tenant_id, group_id, owner_actor_id, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, name, tenant, nullString(strings.TrimSpace(data.GroupID)), nullString(strings.TrimSpace(data.OwnerID)), nullJSON(data.Metadata), now, now)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *DB) ListSessions(ctx context.Context, limit, offset int) ([]byte, int, error) {
	limit, offset = clampPagination(limit, offset)
	query := `WITH total AS (SELECT COUNT(*) AS cnt FROM sessions)
	SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			'session_id', session_id,
			'name', name,
			'tenant_id', tenant_id,
			'group_id', group_id,
			'owner_actor_id', owner_actor_id,
			'metadata', metadata,
			'created_at', created_at,
			'updated_at', updated_at
		) ORDER BY created_at DESC
	), '[]'::jsonb),
	(SELECT cnt FROM total)
	FROM (SELECT * FROM sessions ORDER BY created_at DESC LIMIT $1 OFFSET $2) AS sub`
	row := d.conn.QueryRowContext(ctx, query, limit, offset)
	var out []byte
	var total int
	if err := row.Scan(&out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (d *DB) GetSession(ctx context.Context, sessionID string) ([]byte, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("session_id required")
	}
	row := d.conn.QueryRowContext(ctx, `
		SELECT name, tenant_id, group_id, owner_actor_id, metadata, created_at, updated_at
		FROM sessions WHERE session_id=$1
	`, sessionID)
	var name, tenant, groupID, owner string
	var metadata []byte
	var createdAt, updatedAt time.Time
	if err := row.Scan(&name, &tenant, &groupID, &owner, &metadata, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	out := map[string]any{
		"session_id":     sessionID,
		"name":           name,
		"tenant_id":      tenant,
		"group_id":       groupID,
		"owner_actor_id": owner,
		"metadata":       json.RawMessage(defaultJSON(metadata)),
		"created_at":     createdAt.UTC().Format(time.RFC3339),
		"updated_at":     updatedAt.UTC().Format(time.RFC3339),
	}
	return json.Marshal(out)
}

func (d *DB) UpdateSession(ctx context.Context, sessionID string, payload []byte) error {
	if d == nil || d.conn == nil {
		return errors.New("db required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("session_id required")
	}
	var data sessionPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}
	name := strings.TrimSpace(data.Name)
	tenant := strings.TrimSpace(data.TenantID)
	if name == "" || tenant == "" {
		return errors.New("name and tenant_id required")
	}
	_, err := d.conn.ExecContext(ctx, `
		UPDATE sessions
		SET name=$2, tenant_id=$3, group_id=$4, owner_actor_id=$5, metadata=$6, updated_at=$7
		WHERE session_id=$1
	`, sessionID, name, tenant, nullString(strings.TrimSpace(data.GroupID)), nullString(strings.TrimSpace(data.OwnerID)), nullJSON(data.Metadata), time.Now().UTC())
	return err
}

func (d *DB) DeleteSession(ctx context.Context, sessionID string) error {
	if d == nil || d.conn == nil {
		return errors.New("db required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("session_id required")
	}
	_, err := d.conn.ExecContext(ctx, `DELETE FROM sessions WHERE session_id=$1`, sessionID)
	return err
}

func (d *DB) AddSessionMember(ctx context.Context, sessionID string, payload []byte) error {
	if d == nil || d.conn == nil {
		return errors.New("db required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("session_id required")
	}
	var data sessionMemberPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}
	member := strings.TrimSpace(data.MemberID)
	role := strings.TrimSpace(data.Role)
	if member == "" || role == "" {
		return errors.New("member_id and role required")
	}
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO session_members(session_id, member_id, role, added_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (session_id, member_id) DO UPDATE SET role=excluded.role
	`, sessionID, member, role, time.Now().UTC())
	return err
}

func (d *DB) ListSessionMembers(ctx context.Context, sessionID string) ([]byte, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("session_id required")
	}
	query := `SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			'member_id', member_id,
			'role', role,
			'added_at', added_at
		) ORDER BY added_at DESC
	), '[]'::jsonb) FROM session_members WHERE session_id=$1`
	row := d.conn.QueryRowContext(ctx, query, sessionID)
	var out []byte
	if err := row.Scan(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) IsSessionMember(ctx context.Context, sessionID, memberID string) (bool, error) {
	if d == nil || d.conn == nil {
		return false, errors.New("db required")
	}
	sessionID = strings.TrimSpace(sessionID)
	memberID = strings.TrimSpace(memberID)
	if sessionID == "" || memberID == "" {
		return false, errors.New("session_id and member_id required")
	}
	var exists bool
	err := d.conn.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM session_members WHERE session_id=$1 AND member_id=$2
		)
	`, sessionID, memberID).Scan(&exists)
	return exists, err
}
