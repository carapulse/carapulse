package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":8080"},"policy":{"opa_url":"http://opa","policy_package":"p"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadConfig(file); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestLoadConfigBadJSON(t *testing.T) {
	file := t.TempDir() + "/cfg.json"
	if err := os.WriteFile(file, []byte("{"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadConfig(file); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	if _, err := LoadConfig("/no/such/file.json"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadConfigInvalidContent(t *testing.T) {
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":8080"}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadConfig(file); err == nil {
		t.Fatalf("expected error")
	}
}
