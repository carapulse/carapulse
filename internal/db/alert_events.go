package db

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (d *DB) UpsertAlertEvent(ctx context.Context, fingerprint, status string, startedAt time.Time, payload []byte) (string, error) {
	if d == nil || d.conn == nil {
		return "", errors.New("db required")
	}
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return "", errors.New("fingerprint required")
	}
	alertID := fingerprint
	if status == "" {
		status = "firing"
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO alert_events(alert_id, fingerprint, status, started_at, updated_at, payload_json)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (alert_id) DO UPDATE
		SET status=excluded.status,
		    updated_at=excluded.updated_at,
		    payload_json=excluded.payload_json
	`, alertID, fingerprint, status, startedAt, time.Now().UTC(), payload)
	if err != nil {
		return "", err
	}
	return alertID, nil
}
