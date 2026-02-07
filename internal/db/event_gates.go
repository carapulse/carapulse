package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type EventGateState struct {
	FirstSeen       time.Time
	LastSeen        time.Time
	Count           int
	SuppressedUntil sql.NullTime
}

func (d *DB) UpsertEventGate(ctx context.Context, source, fingerprint string, now time.Time, window, backoff time.Duration, minCount int) (bool, EventGateState, error) {
	if d == nil || d.conn == nil {
		return false, EventGateState{}, errors.New("db required")
	}
	source = strings.TrimSpace(source)
	fingerprint = strings.TrimSpace(fingerprint)
	if source == "" || fingerprint == "" {
		return false, EventGateState{}, errors.New("source and fingerprint required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var state EventGateState
	err := d.conn.QueryRowContext(ctx, `
		SELECT first_seen, last_seen, count, suppressed_until
		FROM event_gates WHERE source=$1 AND fingerprint=$2
	`, source, fingerprint).Scan(&state.FirstSeen, &state.LastSeen, &state.Count, &state.SuppressedUntil)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return false, EventGateState{}, err
		}
		state = EventGateState{FirstSeen: now, LastSeen: now, Count: 1}
		allowed := minCount <= 1
		var suppressed any
		if allowed && backoff > 0 {
			suppressed = now.Add(backoff)
			state.SuppressedUntil = sql.NullTime{Time: now.Add(backoff), Valid: true}
		}
		_, err = d.conn.ExecContext(ctx, `
			INSERT INTO event_gates(source, fingerprint, first_seen, last_seen, count, suppressed_until)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, source, fingerprint, state.FirstSeen, state.LastSeen, state.Count, suppressed)
		return allowed, state, err
	}
	if state.SuppressedUntil.Valid && now.Before(state.SuppressedUntil.Time) {
		state.LastSeen = now
		state.Count++
		_, err = d.conn.ExecContext(ctx, `
			UPDATE event_gates SET last_seen=$3, count=$4 WHERE source=$1 AND fingerprint=$2
		`, source, fingerprint, state.LastSeen, state.Count)
		return false, state, err
	}
	if window > 0 && now.Sub(state.FirstSeen) > window {
		state.FirstSeen = now
		state.Count = 0
		state.SuppressedUntil = sql.NullTime{}
	}
	state.LastSeen = now
	state.Count++
	allowed := minCount <= 1 || state.Count >= minCount
	var suppressed any
	if allowed && backoff > 0 {
		state.SuppressedUntil = sql.NullTime{Time: now.Add(backoff), Valid: true}
		suppressed = state.SuppressedUntil.Time
	}
	_, err = d.conn.ExecContext(ctx, `
		UPDATE event_gates
		SET first_seen=$3, last_seen=$4, count=$5, suppressed_until=$6
		WHERE source=$1 AND fingerprint=$2
	`, source, fingerprint, state.FirstSeen, state.LastSeen, state.Count, suppressed)
	return allowed, state, err
}
