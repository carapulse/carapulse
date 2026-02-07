package secrets

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveTemplatePaths(t *testing.T) {
	source, dest := ResolveTemplatePaths("/tmp/templates", "", "")
	if source == "" || dest == "" {
		t.Fatalf("expected defaults")
	}
	expSource := filepath.Join("/tmp/templates", "vault-agent.ctmpl")
	if source != expSource {
		t.Fatalf("source: %s", source)
	}
}

func TestBuildVaultAgentConfigFromConnectors(t *testing.T) {
	cfg, err := BuildVaultAgentConfigFromConnectors("http://vault", "", "", "role", "", "", "", "/tmp/token", "", "", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cfg.AutoAuthMethod != "kubernetes" {
		t.Fatalf("method: %s", cfg.AutoAuthMethod)
	}
	if cfg.SinkPath != "/tmp/token" {
		t.Fatalf("sink: %s", cfg.SinkPath)
	}
}

func TestParseSessionID(t *testing.T) {
	id, err := ParseSessionID([]byte(`{"id":"s1"}`))
	if err != nil || id != "s1" {
		t.Fatalf("id: %s err: %v", id, err)
	}
}

func TestStartVaultAgentNoAddr(t *testing.T) {
	handle, err := StartVaultAgent(context.Background(), VaultAgentConfig{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle != nil {
		t.Fatalf("expected nil handle")
	}
}

func TestStartVaultAgentLookPathError(t *testing.T) {
	oldLook := lookPath
	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	defer func() { lookPath = oldLook }()
	_, err := StartVaultAgent(context.Background(), VaultAgentConfig{VaultAddr: "http://vault", SinkPath: "/tmp/token"})
	if err == nil {
		t.Fatalf("expected error")
	}
}
