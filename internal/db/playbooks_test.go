package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCreatePlaybook(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{
		"tenant_id": "tenant",
		"name":      "deploy",
		"version":   1,
		"tags":      []string{"sre"},
		"spec":      map[string]any{"steps": []any{}},
	})
	if _, err := d.CreatePlaybook(context.Background(), payload); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "INSERT INTO playbooks") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestListPlaybooks(t *testing.T) {
	conn := &fakeConn{row: fakeRow{values: []any{[]byte(`[]`), 0}}}
	d := &DB{conn: conn}
	out, _, err := d.ListPlaybooks(context.Background(), 50, 0)
	if err != nil || string(out) != "[]" {
		t.Fatalf("out: %s err: %v", string(out), err)
	}
}

func TestGetPlaybookNotFound(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrNoRows}}
	d := &DB{conn: conn}
	out, err := d.GetPlaybook(context.Background(), "pb")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil")
	}
}

func TestGetPlaybookOK(t *testing.T) {
	now := time.Now().UTC()
	conn := &fakeConn{row: fakeRow{values: []any{"tenant", "playbook", 1, []byte(`["tag"]`), []byte(`{"steps":[]}`), now}}}
	d := &DB{conn: conn}
	out, err := d.GetPlaybook(context.Background(), "pb")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(string(out), "\"playbook_id\"") {
		t.Fatalf("out: %s", string(out))
	}
}

func TestDeletePlaybook(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.DeletePlaybook(context.Background(), "pb_1"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "DELETE FROM playbooks") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestDeletePlaybookNilDB(t *testing.T) {
	var d *DB
	if err := d.DeletePlaybook(context.Background(), "pb_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeletePlaybookExecError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	if err := d.DeletePlaybook(context.Background(), "pb_1"); err == nil {
		t.Fatalf("expected error")
	}
}
