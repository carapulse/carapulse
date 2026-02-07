package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCreateRunbook(t *testing.T) {
	conn := &fakeConn{row: fakeRow{values: []any{0}}}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{
		"tenant_id": "tenant",
		"service":   "api",
		"name":      "deploy",
		"tags":      []string{"sre"},
		"spec":      map[string]any{"steps": []any{}},
	})
	if _, err := d.CreateRunbook(context.Background(), payload); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "INSERT INTO runbooks") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
	if !strings.Contains(conn.lastQuery, "tenant_id=$1") {
		t.Fatalf("version query should scope by tenant_id: %s", conn.lastQuery)
	}
}

func TestListRunbooks(t *testing.T) {
	conn := &fakeConn{row: fakeRow{values: []any{[]byte(`[]`)}}}
	d := &DB{conn: conn}
	out, err := d.ListRunbooks(context.Background())
	if err != nil || string(out) != "[]" {
		t.Fatalf("out: %s err: %v", string(out), err)
	}
}

func TestGetRunbookNotFound(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrNoRows}}
	d := &DB{conn: conn}
	if _, err := d.GetRunbook(context.Background(), "rb"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetRunbookOK(t *testing.T) {
	now := time.Now().UTC()
	conn := &fakeConn{row: fakeRow{values: []any{"rb", "tenant", "api", "deploy", 1, []byte(`["tag"]`), "body", []byte(`{"steps":[]}`), now}}}
	d := &DB{conn: conn}
	out, err := d.GetRunbook(context.Background(), "rb")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(string(out), "\"runbook_id\"") {
		t.Fatalf("out: %s", string(out))
	}
}
