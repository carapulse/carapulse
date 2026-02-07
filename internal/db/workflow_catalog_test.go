package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateWorkflowCatalog(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{
		"tenant_id": "t1",
		"name":      "deploy-workflow",
		"version":   1,
		"spec":      map[string]any{"steps": []any{"deploy", "verify"}},
	})
	id, err := d.CreateWorkflowCatalog(context.Background(), payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
	if !strings.HasPrefix(id, "workflow_") {
		t.Fatalf("unexpected id prefix: %s", id)
	}
	if !strings.Contains(conn.lastExecQuery, "INSERT INTO workflow_catalog") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestCreateWorkflowCatalogDefaultVersion(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{
		"name": "scale-workflow",
		"spec": map[string]any{"steps": []any{"scale"}},
	})
	id, err := d.CreateWorkflowCatalog(context.Background(), payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
	// version 0 should default to 1
	if len(conn.lastExecArgs) < 4 {
		t.Fatalf("missing args")
	}
	if conn.lastExecArgs[3] != 1 {
		t.Fatalf("expected version 1, got: %v", conn.lastExecArgs[3])
	}
}

func TestCreateWorkflowCatalogMissingName(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{
		"spec": map[string]any{"steps": []any{}},
	})
	if _, err := d.CreateWorkflowCatalog(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateWorkflowCatalogMissingSpec(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload, _ := json.Marshal(map[string]any{
		"name":    "wf",
		"version": 1,
	})
	if _, err := d.CreateWorkflowCatalog(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateWorkflowCatalogNilDB(t *testing.T) {
	var d *DB
	payload, _ := json.Marshal(map[string]any{
		"name": "wf",
		"spec": map[string]any{"steps": []any{}},
	})
	if _, err := d.CreateWorkflowCatalog(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateWorkflowCatalogNoConn(t *testing.T) {
	d := &DB{}
	payload, _ := json.Marshal(map[string]any{
		"name": "wf",
		"spec": map[string]any{"steps": []any{}},
	})
	if _, err := d.CreateWorkflowCatalog(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateWorkflowCatalogInvalidJSON(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.CreateWorkflowCatalog(context.Background(), []byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateWorkflowCatalogExecError(t *testing.T) {
	conn := &fakeConn{execErr: errTest}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{
		"name": "wf",
		"spec": map[string]any{"steps": []any{}},
	})
	if _, err := d.CreateWorkflowCatalog(context.Background(), payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateWorkflowCatalogNoTenant(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload, _ := json.Marshal(map[string]any{
		"name":    "wf",
		"version": 2,
		"spec":    map[string]any{"steps": []any{}},
	})
	id, err := d.CreateWorkflowCatalog(context.Background(), payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
	// nullString("") returns nil for empty tenant
	if conn.lastExecArgs[1] != nil {
		t.Fatalf("expected nil tenant_id, got: %v", conn.lastExecArgs[1])
	}
}

func TestListWorkflowCatalog(t *testing.T) {
	conn := &fakeConn{row: fakeRow{values: []any{[]byte(`[]`)}}}
	d := &DB{conn: conn}
	out, err := d.ListWorkflowCatalog(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != "[]" {
		t.Fatalf("out: %s", out)
	}
}

func TestListWorkflowCatalogScanError(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrConnDone}}
	d := &DB{conn: conn}
	if _, err := d.ListWorkflowCatalog(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListWorkflowCatalogWithData(t *testing.T) {
	data := `[{"workflow_id":"wf_1","name":"deploy"}]`
	conn := &fakeConn{row: fakeRow{values: []any{[]byte(data)}}}
	d := &DB{conn: conn}
	out, err := d.ListWorkflowCatalog(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != data {
		t.Fatalf("out: %s", out)
	}
}
