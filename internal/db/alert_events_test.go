package db

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestUpsertAlertEvent(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	now := time.Now().UTC()
	payload := []byte(`{"alertname":"HighLatency","severity":"critical"}`)
	id, err := d.UpsertAlertEvent(context.Background(), "fp123", "firing", now, payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "fp123" {
		t.Fatalf("expected alertID == fingerprint, got: %s", id)
	}
	if !strings.Contains(conn.lastExecQuery, "INSERT INTO alert_events") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
	if !strings.Contains(conn.lastExecQuery, "ON CONFLICT") {
		t.Fatalf("expected upsert, query: %s", conn.lastExecQuery)
	}
}

func TestUpsertAlertEventDefaultStatus(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	now := time.Now().UTC()
	id, err := d.UpsertAlertEvent(context.Background(), "fp456", "", now, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "fp456" {
		t.Fatalf("id: %s", id)
	}
	// status defaults to "firing"
	if conn.lastExecArgs[2] != "firing" {
		t.Fatalf("expected default status 'firing', got: %v", conn.lastExecArgs[2])
	}
}

func TestUpsertAlertEventZeroTime(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	id, err := d.UpsertAlertEvent(context.Background(), "fp789", "resolved", time.Time{}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "fp789" {
		t.Fatalf("id: %s", id)
	}
	// startedAt should default to a non-zero time
	if startedAt, ok := conn.lastExecArgs[3].(time.Time); ok {
		if startedAt.IsZero() {
			t.Fatalf("expected non-zero startedAt")
		}
	}
}

func TestUpsertAlertEventMissingFingerprint(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.UpsertAlertEvent(context.Background(), "", "firing", time.Now(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertAlertEventWhitespaceFingerprint(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.UpsertAlertEvent(context.Background(), "   ", "firing", time.Now(), nil); err == nil {
		t.Fatalf("expected error for whitespace-only fingerprint")
	}
}

func TestUpsertAlertEventNilDB(t *testing.T) {
	var d *DB
	if _, err := d.UpsertAlertEvent(context.Background(), "fp", "firing", time.Now(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertAlertEventNoConn(t *testing.T) {
	d := &DB{}
	if _, err := d.UpsertAlertEvent(context.Background(), "fp", "firing", time.Now(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertAlertEventExecError(t *testing.T) {
	conn := &fakeConn{execErr: errTest}
	d := &DB{conn: conn}
	if _, err := d.UpsertAlertEvent(context.Background(), "fp", "firing", time.Now(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertAlertEventNilPayload(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	id, err := d.UpsertAlertEvent(context.Background(), "fp_nil", "firing", time.Now(), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "fp_nil" {
		t.Fatalf("id: %s", id)
	}
}

func TestUpsertAlertEventResolvedStatus(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	id, err := d.UpsertAlertEvent(context.Background(), "fp_res", "resolved", time.Now(), []byte(`{}`))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "fp_res" {
		t.Fatalf("id: %s", id)
	}
	if conn.lastExecArgs[2] != "resolved" {
		t.Fatalf("status: %v", conn.lastExecArgs[2])
	}
}

func TestUpsertAlertEventExecArgs(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	payload := []byte(`{"key":"val"}`)
	if _, err := d.UpsertAlertEvent(context.Background(), "fp_args", "firing", now, payload); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(conn.lastExecArgs) != 6 {
		t.Fatalf("expected 6 args, got: %d", len(conn.lastExecArgs))
	}
	// arg0: alert_id, arg1: fingerprint, arg2: status, arg3: started_at, arg4: updated_at, arg5: payload
	if conn.lastExecArgs[0] != "fp_args" {
		t.Fatalf("alert_id: %v", conn.lastExecArgs[0])
	}
	if conn.lastExecArgs[1] != "fp_args" {
		t.Fatalf("fingerprint: %v", conn.lastExecArgs[1])
	}
	if conn.lastExecArgs[2] != "firing" {
		t.Fatalf("status: %v", conn.lastExecArgs[2])
	}
}
