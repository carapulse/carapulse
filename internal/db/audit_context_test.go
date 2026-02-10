package db

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestListAuditEvents(t *testing.T) {
	row := fakeRow{values: []any{[]byte(`[]`), 0}}
	conn := &fakeConn{row: row}
	d := &DB{conn: conn}
	out, total, err := d.ListAuditEvents(context.Background(), AuditFilter{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != "[]" {
		t.Fatalf("unexpected output: %s", string(out))
	}
	if total != 0 {
		t.Fatalf("total: got %d, want 0", total)
	}
}

func TestListAuditEventsFilters(t *testing.T) {
	row := fakeRow{values: []any{[]byte(`[]`), 0}}
	conn := &fakeConn{row: row}
	d := &DB{conn: conn}
	filter := AuditFilter{
		From:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		To:       time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		ActorID:  "actor",
		Action:   "deploy",
		Decision: "allow",
	}
	if _, _, err := d.ListAuditEvents(context.Background(), filter); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastQuery, "occurred_at >=") {
		t.Fatalf("missing from filter")
	}
	if !strings.Contains(conn.lastQuery, "occurred_at <=") {
		t.Fatalf("missing to filter")
	}
	if !strings.Contains(conn.lastQuery, "actor_json->>'id'") {
		t.Fatalf("missing actor filter")
	}
	if !strings.Contains(conn.lastQuery, "action =") {
		t.Fatalf("missing action filter")
	}
	if !strings.Contains(conn.lastQuery, "decision =") {
		t.Fatalf("missing decision filter")
	}
	if got := len(conn.lastArgs); got != 7 {
		t.Fatalf("args: got %d, want 7 (5 filters + limit + offset)", got)
	}
}

func TestListAuditEventsRowError(t *testing.T) {
	row := fakeRow{err: sql.ErrConnDone}
	conn := &fakeConn{row: row}
	d := &DB{conn: conn}
	if _, _, err := d.ListAuditEvents(context.Background(), AuditFilter{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListContextServices(t *testing.T) {
	row := fakeRow{values: []any{[]byte(`[]`), 0}}
	conn := &fakeConn{row: row}
	d := &DB{conn: conn}
	out, total, err := d.ListContextServices(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != "[]" {
		t.Fatalf("unexpected output: %s", string(out))
	}
	if total != 0 {
		t.Fatalf("total: got %d, want 0", total)
	}
	if !strings.Contains(conn.lastQuery, "context_nodes") {
		t.Fatalf("missing query")
	}
}

func TestListContextServicesRowError(t *testing.T) {
	row := fakeRow{err: sql.ErrConnDone}
	conn := &fakeConn{row: row}
	d := &DB{conn: conn}
	if _, _, err := d.ListContextServices(context.Background(), 50, 0); err == nil {
		t.Fatalf("expected error")
	}
}
