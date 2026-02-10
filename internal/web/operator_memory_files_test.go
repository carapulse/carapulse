package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOperatorMemoryEmpty(t *testing.T) {
	entries, err := LoadOperatorMemory("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil")
	}
}

func TestLoadOperatorMemoryMissingFile(t *testing.T) {
	entries, err := LoadOperatorMemory(t.TempDir())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil")
	}
}

func TestLoadOperatorMemoryValid(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	entries := []OperatorMemoryRequest{
		{TenantID: "t1", Title: "title1", Body: "body1", Tags: []string{"tag1"}},
		{TenantID: "t2", Title: "title2", Body: "body2"},
	}
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(filepath.Join(memDir, "operator_memory.json"), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := LoadOperatorMemory(dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("len: %d", len(loaded))
	}
	if loaded[0].Title != "title1" || loaded[1].TenantID != "t2" {
		t.Fatalf("entries: %#v", loaded)
	}
}

func TestLoadOperatorMemoryInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "operator_memory.json"), []byte("{"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadOperatorMemory(dir); err == nil {
		t.Fatalf("expected error")
	}
}

func TestFileMemoryID(t *testing.T) {
	req := OperatorMemoryRequest{TenantID: "t", Title: "test", Body: "body"}
	id := fileMemoryID(req)
	if id == "" {
		t.Fatalf("empty id")
	}
	if id[:5] != "file_" {
		t.Fatalf("prefix: %s", id)
	}
	// Same input produces same ID
	id2 := fileMemoryID(req)
	if id != id2 {
		t.Fatalf("non-deterministic: %s vs %s", id, id2)
	}
	// Different input produces different ID
	req2 := OperatorMemoryRequest{TenantID: "t", Title: "other", Body: "body"}
	id3 := fileMemoryID(req2)
	if id == id3 {
		t.Fatalf("collision: %s", id)
	}
}
