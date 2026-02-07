package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCreateSession(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{"name": "test", "tenant_id": "t1"})
	id, err := d.CreateSession(context.Background(), payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
	if !strings.Contains(conn.lastExecQuery, "INSERT INTO sessions") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestCreateSessionWithOptionalFields(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{
		"name":           "test",
		"tenant_id":      "t1",
		"group_id":       "g1",
		"owner_actor_id": "o1",
		"metadata":       map[string]any{"key": "value"},
	})
	id, err := d.CreateSession(context.Background(), payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
	if !strings.HasPrefix(id, "session_") {
		t.Fatalf("unexpected id prefix: %s", id)
	}
}

func TestCreateSessionMissingName(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{"tenant_id": "t1"})
	if _, err := d.CreateSession(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateSessionMissingTenant(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{"name": "test"})
	if _, err := d.CreateSession(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateSessionNilDB(t *testing.T) {
	var d *DB
	payload, _ := json.Marshal(map[string]any{"name": "test", "tenant_id": "t1"})
	if _, err := d.CreateSession(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateSessionNoConn(t *testing.T) {
	d := &DB{}
	payload, _ := json.Marshal(map[string]any{"name": "test", "tenant_id": "t1"})
	if _, err := d.CreateSession(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateSessionInvalidJSON(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.CreateSession(context.Background(), []byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateSessionExecError(t *testing.T) {
	conn := &fakeConn{execErr: errTest}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{"name": "test", "tenant_id": "t1"})
	if _, err := d.CreateSession(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateSessionWhitespaceOnly(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{"name": "  ", "tenant_id": "t1"})
	if _, err := d.CreateSession(context.Background(), payload); err == nil {
		t.Fatalf("expected error for whitespace-only name")
	}
}

func TestListSessions(t *testing.T) {
	conn := &fakeConn{row: fakeRow{values: []any{[]byte("[]")}}}
	d := &DB{conn: conn}
	out, err := d.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != "[]" {
		t.Fatalf("out: %s", out)
	}
}

func TestListSessionsScanError(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrConnDone}}
	d := &DB{conn: conn}
	if _, err := d.ListSessions(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetSession(t *testing.T) {
	now := time.Now().UTC()
	conn := &fakeConn{row: fakeRow{values: []any{
		"test", "t1", "", "", []byte("{}"), now, now,
	}}}
	d := &DB{conn: conn}
	out, err := d.GetSession(context.Background(), "session_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("nil result")
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["session_id"] != "session_1" {
		t.Fatalf("session_id: %v", decoded["session_id"])
	}
	if decoded["name"] != "test" {
		t.Fatalf("name: %v", decoded["name"])
	}
}

func TestGetSessionNotFound(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrNoRows}}
	d := &DB{conn: conn}
	out, err := d.GetSession(context.Background(), "missing")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil")
	}
}

func TestGetSessionEmptyID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.GetSession(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetSessionScanError(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrConnDone}}
	d := &DB{conn: conn}
	if _, err := d.GetSession(context.Background(), "session_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateSession(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{"name": "updated", "tenant_id": "t1"})
	if err := d.UpdateSession(context.Background(), "session_1", payload); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "UPDATE sessions") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestUpdateSessionEmptyID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{"name": "x", "tenant_id": "t"})
	if err := d.UpdateSession(context.Background(), "", payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateSessionNilDB(t *testing.T) {
	var d *DB
	payload, _ := json.Marshal(map[string]any{"name": "x", "tenant_id": "t"})
	if err := d.UpdateSession(context.Background(), "session_1", payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateSessionInvalidJSON(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.UpdateSession(context.Background(), "session_1", []byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateSessionMissingFields(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{"name": "x"})
	if err := d.UpdateSession(context.Background(), "session_1", payload); err == nil {
		t.Fatalf("expected error for missing tenant_id")
	}
}

func TestUpdateSessionExecError(t *testing.T) {
	conn := &fakeConn{execErr: errTest}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{"name": "x", "tenant_id": "t"})
	if err := d.UpdateSession(context.Background(), "session_1", payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteSession(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.DeleteSession(context.Background(), "session_1"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "DELETE FROM sessions") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestDeleteSessionEmptyID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.DeleteSession(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteSessionNilDB(t *testing.T) {
	var d *DB
	if err := d.DeleteSession(context.Background(), "session_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteSessionExecError(t *testing.T) {
	conn := &fakeConn{execErr: errTest}
	d := &DB{conn: conn}
	if err := d.DeleteSession(context.Background(), "session_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAddSessionMember(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{"member_id": "m1", "role": "viewer"})
	if err := d.AddSessionMember(context.Background(), "session_1", payload); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "INSERT INTO session_members") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestAddSessionMemberMissingRole(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{"member_id": "m1"})
	if err := d.AddSessionMember(context.Background(), "session_1", payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAddSessionMemberMissingMemberID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{"role": "viewer"})
	if err := d.AddSessionMember(context.Background(), "session_1", payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAddSessionMemberEmptySessionID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{"member_id": "m1", "role": "viewer"})
	if err := d.AddSessionMember(context.Background(), "", payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAddSessionMemberNilDB(t *testing.T) {
	var d *DB
	payload, _ := json.Marshal(map[string]any{"member_id": "m1", "role": "viewer"})
	if err := d.AddSessionMember(context.Background(), "session_1", payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAddSessionMemberInvalidJSON(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.AddSessionMember(context.Background(), "session_1", []byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAddSessionMemberExecError(t *testing.T) {
	conn := &fakeConn{execErr: errTest}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{"member_id": "m1", "role": "viewer"})
	if err := d.AddSessionMember(context.Background(), "session_1", payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListSessionMembers(t *testing.T) {
	conn := &fakeConn{row: fakeRow{values: []any{[]byte("[]")}}}
	d := &DB{conn: conn}
	out, err := d.ListSessionMembers(context.Background(), "session_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != "[]" {
		t.Fatalf("out: %s", out)
	}
}

func TestListSessionMembersEmptyID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.ListSessionMembers(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListSessionMembersScanError(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrConnDone}}
	d := &DB{conn: conn}
	if _, err := d.ListSessionMembers(context.Background(), "session_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestIsSessionMember(t *testing.T) {
	conn := &fakeConn{row: fakeRow{values: []any{true}}}
	d := &DB{conn: conn}
	exists, err := d.IsSessionMember(context.Background(), "session_1", "m1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !exists {
		t.Fatalf("expected true")
	}
}

func TestIsSessionMemberFalse(t *testing.T) {
	conn := &fakeConn{row: fakeRow{values: []any{false}}}
	d := &DB{conn: conn}
	exists, err := d.IsSessionMember(context.Background(), "session_1", "m1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if exists {
		t.Fatalf("expected false")
	}
}

func TestIsSessionMemberEmptySessionID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.IsSessionMember(context.Background(), "", "m1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestIsSessionMemberEmptyMemberID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.IsSessionMember(context.Background(), "session_1", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestIsSessionMemberNilDB(t *testing.T) {
	var d *DB
	if _, err := d.IsSessionMember(context.Background(), "session_1", "m1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestIsSessionMemberScanError(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrConnDone}}
	d := &DB{conn: conn}
	if _, err := d.IsSessionMember(context.Background(), "session_1", "m1"); err == nil {
		t.Fatalf("expected error")
	}
}
