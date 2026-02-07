package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTemplateEnvOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env")
	if err := os.WriteFile(path, []byte("TOKEN=abc\n#comment\nUSER=svc\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	env, err := LoadTemplateEnv(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if env["TOKEN"] != "abc" || env["USER"] != "svc" {
		t.Fatalf("env: %#v", env)
	}
}

func TestLoadTemplateEnvBlankPath(t *testing.T) {
	env, err := LoadTemplateEnv("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if env != nil {
		t.Fatalf("expected nil")
	}
}

func TestLoadTemplateEnvNoEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env")
	if err := os.WriteFile(path, []byte("#only\n\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadTemplateEnv(path); err == nil {
		t.Fatalf("expected error")
	}
}
