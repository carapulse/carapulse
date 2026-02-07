package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCreateOperatorMemory(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{
		"tenant_id": "t1",
		"title":     "runbook note",
		"body":      "some body text",
	})
	id, err := d.CreateOperatorMemory(context.Background(), payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
	if !strings.HasPrefix(id, "memory_") {
		t.Fatalf("unexpected id prefix: %s", id)
	}
	if !strings.Contains(conn.lastExecQuery, "INSERT INTO operator_memory") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestCreateOperatorMemoryWithOptionalFields(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{
		"tenant_id":      "t1",
		"title":          "note",
		"body":           "body",
		"tags":           []string{"sre", "incident"},
		"metadata":       map[string]any{"source": "manual"},
		"owner_actor_id": "actor1",
	})
	id, err := d.CreateOperatorMemory(context.Background(), payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
}

func TestCreateOperatorMemoryMissingTenant(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{"title": "note", "body": "body"})
	if _, err := d.CreateOperatorMemory(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateOperatorMemoryMissingTitle(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{"tenant_id": "t1", "body": "body"})
	if _, err := d.CreateOperatorMemory(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateOperatorMemoryMissingBody(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{"tenant_id": "t1", "title": "note"})
	if _, err := d.CreateOperatorMemory(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateOperatorMemoryNilDB(t *testing.T) {
	var d *DB
	payload, _ := json.Marshal(map[string]any{"tenant_id": "t1", "title": "note", "body": "body"})
	if _, err := d.CreateOperatorMemory(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateOperatorMemoryNoConn(t *testing.T) {
	d := &DB{}
	payload, _ := json.Marshal(map[string]any{"tenant_id": "t1", "title": "note", "body": "body"})
	if _, err := d.CreateOperatorMemory(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateOperatorMemoryInvalidJSON(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.CreateOperatorMemory(context.Background(), []byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateOperatorMemoryExecError(t *testing.T) {
	conn := &fakeConn{execErr: errTest}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{"tenant_id": "t1", "title": "note", "body": "body"})
	if _, err := d.CreateOperatorMemory(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListOperatorMemory(t *testing.T) {
	conn := &fakeConn{row: fakeRow{values: []any{[]byte("[]")}}}
	d := &DB{conn: conn}
	out, err := d.ListOperatorMemory(context.Background(), "t1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != "[]" {
		t.Fatalf("out: %s", out)
	}
}

func TestListOperatorMemoryEmptyTenant(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.ListOperatorMemory(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListOperatorMemoryNilDB(t *testing.T) {
	var d *DB
	if _, err := d.ListOperatorMemory(context.Background(), "t1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListOperatorMemoryScanError(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrConnDone}}
	d := &DB{conn: conn}
	if _, err := d.ListOperatorMemory(context.Background(), "t1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetOperatorMemory(t *testing.T) {
	now := time.Now().UTC()
	conn := &fakeConn{row: fakeRow{values: []any{
		"t1", "my title", "my body", []byte(`["tag1"]`), []byte(`{"key":"val"}`), "actor1", now, now,
	}}}
	d := &DB{conn: conn}
	out, err := d.GetOperatorMemory(context.Background(), "memory_1")
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
	if decoded["memory_id"] != "memory_1" {
		t.Fatalf("memory_id: %v", decoded["memory_id"])
	}
	if decoded["title"] != "my title" {
		t.Fatalf("title: %v", decoded["title"])
	}
	if decoded["body"] != "my body" {
		t.Fatalf("body: %v", decoded["body"])
	}
}

func TestGetOperatorMemoryNotFound(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrNoRows}}
	d := &DB{conn: conn}
	out, err := d.GetOperatorMemory(context.Background(), "missing")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil")
	}
}

func TestGetOperatorMemoryEmptyID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.GetOperatorMemory(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetOperatorMemoryNilDB(t *testing.T) {
	var d *DB
	if _, err := d.GetOperatorMemory(context.Background(), "memory_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetOperatorMemoryScanError(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrConnDone}}
	d := &DB{conn: conn}
	if _, err := d.GetOperatorMemory(context.Background(), "memory_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetOperatorMemoryEmptyMetadata(t *testing.T) {
	now := time.Now().UTC()
	conn := &fakeConn{row: fakeRow{values: []any{
		"t1", "title", "body", []byte(nil), []byte(nil), "", now, now,
	}}}
	d := &DB{conn: conn}
	out, err := d.GetOperatorMemory(context.Background(), "memory_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestUpdateOperatorMemory(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{
		"tenant_id": "t1",
		"title":     "updated title",
		"body":      "updated body",
	})
	if err := d.UpdateOperatorMemory(context.Background(), "memory_1", payload); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "UPDATE operator_memory") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestUpdateOperatorMemoryEmptyID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{"tenant_id": "t", "title": "t", "body": "b"})
	if err := d.UpdateOperatorMemory(context.Background(), "", payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateOperatorMemoryNilDB(t *testing.T) {
	var d *DB
	payload, _ := json.Marshal(map[string]any{"tenant_id": "t", "title": "t", "body": "b"})
	if err := d.UpdateOperatorMemory(context.Background(), "memory_1", payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateOperatorMemoryInvalidJSON(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.UpdateOperatorMemory(context.Background(), "memory_1", []byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateOperatorMemoryMissingFields(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{"tenant_id": "t", "title": "t"})
	if err := d.UpdateOperatorMemory(context.Background(), "memory_1", payload); err == nil {
		t.Fatalf("expected error for missing body")
	}
}

func TestUpdateOperatorMemoryExecError(t *testing.T) {
	conn := &fakeConn{execErr: errTest}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{"tenant_id": "t", "title": "t", "body": "b"})
	if err := d.UpdateOperatorMemory(context.Background(), "memory_1", payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteOperatorMemory(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.DeleteOperatorMemory(context.Background(), "memory_1"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "DELETE FROM operator_memory") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestDeleteOperatorMemoryEmptyID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.DeleteOperatorMemory(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteOperatorMemoryNilDB(t *testing.T) {
	var d *DB
	if err := d.DeleteOperatorMemory(context.Background(), "memory_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteOperatorMemoryExecError(t *testing.T) {
	conn := &fakeConn{execErr: errTest}
	d := &DB{conn: conn}
	if err := d.DeleteOperatorMemory(context.Background(), "memory_1"); err == nil {
		t.Fatalf("expected error")
	}
}
