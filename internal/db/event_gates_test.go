package db

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestUpsertEventGateNewRecord(t *testing.T) {
	// When no existing record (sql.ErrNoRows), a new INSERT is performed
	conn := &fakeConn{
		row: fakeRow{err: sql.ErrNoRows},
	}
	d := &DB{conn: conn}
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	allowed, state, err := d.UpsertEventGate(context.Background(), "alertmanager", "fp1", now, 5*time.Minute, 0, 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed for minCount=1")
	}
	if state.Count != 1 {
		t.Fatalf("count: %d", state.Count)
	}
	if !state.FirstSeen.Equal(now) {
		t.Fatalf("first_seen: %v", state.FirstSeen)
	}
	if !state.LastSeen.Equal(now) {
		t.Fatalf("last_seen: %v", state.LastSeen)
	}
	if !strings.Contains(conn.lastExecQuery, "INSERT INTO event_gates") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestUpsertEventGateNewRecordWithBackoff(t *testing.T) {
	conn := &fakeConn{
		row: fakeRow{err: sql.ErrNoRows},
	}
	d := &DB{conn: conn}
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	backoff := 10 * time.Minute
	allowed, state, err := d.UpsertEventGate(context.Background(), "alertmanager", "fp1", now, 0, backoff, 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed")
	}
	if !state.SuppressedUntil.Valid {
		t.Fatalf("expected suppressed_until to be set")
	}
	expected := now.Add(backoff)
	if !state.SuppressedUntil.Time.Equal(expected) {
		t.Fatalf("suppressed_until: %v, expected: %v", state.SuppressedUntil.Time, expected)
	}
}

func TestUpsertEventGateNewRecordNotAllowed(t *testing.T) {
	// minCount > 1, first event should NOT be allowed
	conn := &fakeConn{
		row: fakeRow{err: sql.ErrNoRows},
	}
	d := &DB{conn: conn}
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	allowed, state, err := d.UpsertEventGate(context.Background(), "src", "fp1", now, 0, 0, 3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if allowed {
		t.Fatalf("expected not allowed for minCount=3")
	}
	if state.Count != 1 {
		t.Fatalf("count: %d", state.Count)
	}
}

func TestUpsertEventGateSuppressed(t *testing.T) {
	// Existing record with suppressed_until in the future
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	suppressedUntil := now.Add(10 * time.Minute)
	conn := &fakeConn{
		row: fakeRow{values: []any{
			now.Add(-5 * time.Minute), // first_seen
			now.Add(-1 * time.Minute), // last_seen
			3,                         // count
			sql.NullTime{Time: suppressedUntil, Valid: true}, // suppressed_until
		}},
	}
	d := &DB{conn: conn}
	allowed, state, err := d.UpsertEventGate(context.Background(), "src", "fp1", now, 0, 0, 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if allowed {
		t.Fatalf("expected suppressed")
	}
	if state.Count != 4 {
		t.Fatalf("count: %d", state.Count)
	}
	if !strings.Contains(conn.lastExecQuery, "UPDATE event_gates") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestUpsertEventGateWindowReset(t *testing.T) {
	// Existing record where window has expired, should reset counters
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	firstSeen := now.Add(-10 * time.Minute) // older than window
	window := 5 * time.Minute
	conn := &fakeConn{
		row: fakeRow{values: []any{
			firstSeen,
			now.Add(-1 * time.Minute),
			5,
			sql.NullTime{}, // not suppressed
		}},
	}
	d := &DB{conn: conn}
	allowed, state, err := d.UpsertEventGate(context.Background(), "src", "fp1", now, window, 0, 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed after window reset")
	}
	// After reset, first_seen=now, count=0+1=1
	if state.Count != 1 {
		t.Fatalf("count: %d, expected 1 after reset", state.Count)
	}
	if !state.FirstSeen.Equal(now) {
		t.Fatalf("first_seen should be reset to now: %v", state.FirstSeen)
	}
}

func TestUpsertEventGateAllowedAfterCountThreshold(t *testing.T) {
	// Existing record where count reaches minCount threshold
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	conn := &fakeConn{
		row: fakeRow{values: []any{
			now.Add(-2 * time.Minute),
			now.Add(-1 * time.Minute),
			2, // current count; after increment, it will be 3 == minCount
			sql.NullTime{},
		}},
	}
	d := &DB{conn: conn}
	allowed, state, err := d.UpsertEventGate(context.Background(), "src", "fp1", now, 10*time.Minute, 0, 3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed when count reaches minCount")
	}
	if state.Count != 3 {
		t.Fatalf("count: %d", state.Count)
	}
}

func TestUpsertEventGateNotYetAtThreshold(t *testing.T) {
	// Existing record where count is still below minCount
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	conn := &fakeConn{
		row: fakeRow{values: []any{
			now.Add(-2 * time.Minute),
			now.Add(-1 * time.Minute),
			1, // current count; after increment, it will be 2 < minCount=3
			sql.NullTime{},
		}},
	}
	d := &DB{conn: conn}
	allowed, _, err := d.UpsertEventGate(context.Background(), "src", "fp1", now, 10*time.Minute, 0, 3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if allowed {
		t.Fatalf("expected not allowed, count below threshold")
	}
}

func TestUpsertEventGateNilDB(t *testing.T) {
	var d *DB
	now := time.Now().UTC()
	if _, _, err := d.UpsertEventGate(context.Background(), "src", "fp", now, 0, 0, 1); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertEventGateNoConn(t *testing.T) {
	d := &DB{}
	now := time.Now().UTC()
	if _, _, err := d.UpsertEventGate(context.Background(), "src", "fp", now, 0, 0, 1); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertEventGateMissingSource(t *testing.T) {
	d := &DB{conn: &fakeConn{row: fakeRow{err: sql.ErrNoRows}}}
	now := time.Now().UTC()
	if _, _, err := d.UpsertEventGate(context.Background(), "", "fp", now, 0, 0, 1); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertEventGateMissingFingerprint(t *testing.T) {
	d := &DB{conn: &fakeConn{row: fakeRow{err: sql.ErrNoRows}}}
	now := time.Now().UTC()
	if _, _, err := d.UpsertEventGate(context.Background(), "src", "", now, 0, 0, 1); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertEventGateZeroTime(t *testing.T) {
	// Zero time should be replaced with time.Now()
	conn := &fakeConn{
		row: fakeRow{err: sql.ErrNoRows},
	}
	d := &DB{conn: conn}
	allowed, state, err := d.UpsertEventGate(context.Background(), "src", "fp", time.Time{}, 0, 0, 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed")
	}
	if state.FirstSeen.IsZero() {
		t.Fatalf("first_seen should not be zero")
	}
}

func TestUpsertEventGateQueryError(t *testing.T) {
	// Error that is not sql.ErrNoRows
	conn := &fakeConn{
		row: fakeRow{err: sql.ErrConnDone},
	}
	d := &DB{conn: conn}
	now := time.Now().UTC()
	if _, _, err := d.UpsertEventGate(context.Background(), "src", "fp", now, 0, 0, 1); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertEventGateInsertExecError(t *testing.T) {
	// No rows found (new record), but INSERT fails
	conn := &fakeConn{
		row:     fakeRow{err: sql.ErrNoRows},
		execErr: errTest,
	}
	d := &DB{conn: conn}
	now := time.Now().UTC()
	if _, _, err := d.UpsertEventGate(context.Background(), "src", "fp", now, 0, 0, 1); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertEventGateUpdateExecError(t *testing.T) {
	// Existing record (suppressed path), but UPDATE fails
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	conn := &fakeConn{
		row: fakeRow{values: []any{
			now.Add(-5 * time.Minute),
			now.Add(-1 * time.Minute),
			3,
			sql.NullTime{Time: now.Add(10 * time.Minute), Valid: true},
		}},
		execErr: errTest,
	}
	d := &DB{conn: conn}
	if _, _, err := d.UpsertEventGate(context.Background(), "src", "fp", now, 0, 0, 1); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertEventGateAllowedWithBackoff(t *testing.T) {
	// Existing record, allowed after suppression expires, with backoff set
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	conn := &fakeConn{
		row: fakeRow{values: []any{
			now.Add(-2 * time.Minute),
			now.Add(-1 * time.Minute),
			2,
			sql.NullTime{}, // not suppressed
		}},
	}
	d := &DB{conn: conn}
	backoff := 15 * time.Minute
	allowed, state, err := d.UpsertEventGate(context.Background(), "src", "fp", now, 10*time.Minute, backoff, 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed")
	}
	if !state.SuppressedUntil.Valid {
		t.Fatalf("expected suppressed_until to be set")
	}
	expected := now.Add(backoff)
	if !state.SuppressedUntil.Time.Equal(expected) {
		t.Fatalf("suppressed_until: %v, expected: %v", state.SuppressedUntil.Time, expected)
	}
}
