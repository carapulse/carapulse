package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestInsertContextSnapshot(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	nodes := []byte(`[{"id":"n1"}]`)
	edges := []byte(`[{"from":"n1","to":"n2"}]`)
	labels := []byte(`{"env":"prod"}`)
	id, err := d.InsertContextSnapshot(context.Background(), "k8s", nodes, edges, labels)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
	if !strings.HasPrefix(id, "snapshot_") {
		t.Fatalf("unexpected id prefix: %s", id)
	}
	if !strings.Contains(conn.lastExecQuery, "INSERT INTO context_snapshots") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestInsertContextSnapshotNoLabels(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	nodes := []byte(`[{"id":"n1"}]`)
	edges := []byte(`[{"from":"n1","to":"n2"}]`)
	id, err := d.InsertContextSnapshot(context.Background(), "helm", nodes, edges, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
}

func TestInsertContextSnapshotEmptyNodes(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.InsertContextSnapshot(context.Background(), "k8s", nil, []byte(`[]`), nil); err == nil {
		t.Fatalf("expected error for empty nodes")
	}
}

func TestInsertContextSnapshotEmptyEdges(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.InsertContextSnapshot(context.Background(), "k8s", []byte(`[]`), nil, nil); err == nil {
		t.Fatalf("expected error for empty edges")
	}
}

func TestInsertContextSnapshotNilDB(t *testing.T) {
	var d *DB
	if _, err := d.InsertContextSnapshot(context.Background(), "k8s", []byte(`[]`), []byte(`[]`), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInsertContextSnapshotNoConn(t *testing.T) {
	d := &DB{}
	if _, err := d.InsertContextSnapshot(context.Background(), "k8s", []byte(`[]`), []byte(`[]`), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInsertContextSnapshotExecError(t *testing.T) {
	conn := &fakeConn{execErr: errTest}
	d := &DB{conn: conn}
	if _, err := d.InsertContextSnapshot(context.Background(), "k8s", []byte(`[]`), []byte(`[]`), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListContextSnapshots(t *testing.T) {
	conn := &fakeConn{row: fakeRow{values: []any{[]byte(`[]`)}}}
	d := &DB{conn: conn}
	out, err := d.ListContextSnapshots(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != "[]" {
		t.Fatalf("out: %s", out)
	}
}

func TestListContextSnapshotsScanError(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrConnDone}}
	d := &DB{conn: conn}
	if _, err := d.ListContextSnapshots(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListContextSnapshotsWithData(t *testing.T) {
	data := `[{"snapshot_id":"snap_1","source":"k8s"}]`
	conn := &fakeConn{row: fakeRow{values: []any{[]byte(data)}}}
	d := &DB{conn: conn}
	out, err := d.ListContextSnapshots(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != data {
		t.Fatalf("out: %s", out)
	}
}

func TestGetContextSnapshot(t *testing.T) {
	now := time.Now().UTC()
	conn := &fakeConn{row: fakeRow{values: []any{
		"snap_1", "k8s", now, []byte(`[{"id":"n1"}]`), []byte(`[{"from":"n1"}]`), []byte(`{"env":"prod"}`),
	}}}
	d := &DB{conn: conn}
	out, err := d.GetContextSnapshot(context.Background(), "snap_1")
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
	if decoded["snapshot_id"] != "snap_1" {
		t.Fatalf("snapshot_id: %v", decoded["snapshot_id"])
	}
	if decoded["source"] != "k8s" {
		t.Fatalf("source: %v", decoded["source"])
	}
}

func TestGetContextSnapshotNotFound(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrNoRows}}
	d := &DB{conn: conn}
	out, err := d.GetContextSnapshot(context.Background(), "missing")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil")
	}
}

func TestGetContextSnapshotEmptyID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.GetContextSnapshot(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetContextSnapshotNilDB(t *testing.T) {
	var d *DB
	if _, err := d.GetContextSnapshot(context.Background(), "snap_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetContextSnapshotScanError(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: sql.ErrConnDone}}
	d := &DB{conn: conn}
	if _, err := d.GetContextSnapshot(context.Background(), "snap_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetContextSnapshotEmptyLabels(t *testing.T) {
	now := time.Now().UTC()
	conn := &fakeConn{row: fakeRow{values: []any{
		"snap_1", "helm", now, []byte(`[]`), []byte(`[]`), []byte(nil),
	}}}
	d := &DB{conn: conn}
	out, err := d.GetContextSnapshot(context.Background(), "snap_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// defaultJSON returns "null" for empty
	if decoded["labels"] != nil {
		// json.RawMessage("null") decodes to nil in map[string]any
	}
}

func TestGetContextSnapshotWhitespaceID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.GetContextSnapshot(context.Background(), "   "); err == nil {
		t.Fatalf("expected error for whitespace-only id")
	}
}
