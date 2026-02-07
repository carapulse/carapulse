package secrets

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderVaultAgentConfigErrors(t *testing.T) {
	if _, err := RenderVaultAgentConfig(VaultAgentConfig{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRenderVaultAgentConfigKubernetes(t *testing.T) {
	cfg := VaultAgentConfig{
		VaultAddr:      "http://vault",
		KubernetesRole: "role",
		SinkPath:       "/tmp/token",
	}
	out, err := RenderVaultAgentConfig(cfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "address = \"http://vault\"") {
		t.Fatalf("missing addr")
	}
	if !strings.Contains(text, "method \"kubernetes\"") {
		t.Fatalf("missing kubernetes method")
	}
	if !strings.Contains(text, "role = \"role\"") {
		t.Fatalf("missing role")
	}
	if !strings.Contains(text, "path = \"/tmp/token\"") {
		t.Fatalf("missing sink")
	}
}

func TestRenderVaultAgentConfigAppRole(t *testing.T) {
	cfg := VaultAgentConfig{
		VaultAddr:       "http://vault",
		Namespace:       "ns",
		AutoAuthMethod:  "approle",
		AppRoleID:       "id",
		AppRoleSecret:   "secret",
		SinkPath:        "/tmp/token",
		TemplateSource:  "/tmp/source",
		TemplateDest:    "/tmp/dest",
		RetryMaxBackoff: "5s",
	}
	out, err := RenderVaultAgentConfig(cfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "namespace = \"ns\"") {
		t.Fatalf("missing namespace")
	}
	if !strings.Contains(text, "method \"approle\"") {
		t.Fatalf("missing approle method")
	}
	if !strings.Contains(text, "role_id = \"id\"") || !strings.Contains(text, "secret_id = \"secret\"") {
		t.Fatalf("missing approle config")
	}
	if !strings.Contains(text, "template {") || !strings.Contains(text, "source = \"/tmp/source\"") {
		t.Fatalf("missing template")
	}
	if !strings.Contains(text, "retry {") || !strings.Contains(text, "max_backoff = \"5s\"") {
		t.Fatalf("missing retry")
	}
}

func TestVaultAgentHelpers(t *testing.T) {
	if _, err := BuildVaultAgentConfigFromConnectors("", "", "", "", "", "", "", "", "", "", ""); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := BuildVaultAgentConfigFromConnectors("http://vault", "", "", "", "", "", "", "/tmp/token", "", "/tmp/dest", ""); err == nil {
		t.Fatalf("expected template error")
	}
	cfg, err := BuildVaultAgentConfigFromConnectors("http://vault", "", "", "role", "auth/approle", "id", "secret", "/tmp/token", "", "", "10s")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cfg.AutoAuthMethod != "approle" {
		t.Fatalf("method: %s", cfg.AutoAuthMethod)
	}
	if got := FormatBoundarySessionEnv(""); got != nil {
		t.Fatalf("expected nil env")
	}
	env := FormatBoundarySessionEnv("sess")
	if env["BOUNDARY_SESSION_ID"] != "sess" {
		t.Fatalf("env: %#v", env)
	}
	if got := BuildBoundarySessionDuration(" 5m "); got != "5m" {
		t.Fatalf("duration: %s", got)
	}
	desc := DescribeVaultAgent(VaultAgentConfig{VaultAddr: "http://vault", AutoAuthMethod: "kubernetes", SinkPath: "/tmp/token"})
	if !strings.Contains(desc, "vault=http://vault") {
		t.Fatalf("desc: %s", desc)
	}
}

func TestStartVaultAgentNoSink(t *testing.T) {
	handle, err := StartVaultAgent(context.Background(), VaultAgentConfig{VaultAddr: "http://vault"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle != nil {
		t.Fatalf("expected nil handle")
	}
}

func TestStartVaultAgentTempDirError(t *testing.T) {
	oldLook := lookPath
	oldCreate := createTempDir
	defer func() {
		lookPath = oldLook
		createTempDir = oldCreate
	}()
	lookPath = func(string) (string, error) { return "/bin/true", nil }
	createTempDir = func(string, string) (string, error) { return "", errors.New("boom") }
	if _, err := StartVaultAgent(context.Background(), VaultAgentConfig{VaultAddr: "http://vault", SinkPath: "/tmp/token"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestStartVaultAgentWriteError(t *testing.T) {
	oldLook := lookPath
	oldCreate := createTempDir
	oldWrite := writeFile
	defer func() {
		lookPath = oldLook
		createTempDir = oldCreate
		writeFile = oldWrite
	}()
	lookPath = func(string) (string, error) { return "/bin/true", nil }
	createTempDir = func(string, string) (string, error) { return t.TempDir(), nil }
	writeFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	if _, err := StartVaultAgent(context.Background(), VaultAgentConfig{VaultAddr: "http://vault", SinkPath: "/tmp/token"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestStartVaultAgentSuccess(t *testing.T) {
	tmp := t.TempDir()
	vaultPath := filepath.Join(tmp, "vault")
	if err := os.WriteFile(vaultPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("PATH", tmp+string(os.PathListSeparator)+os.Getenv("PATH"))
	oldLook := lookPath
	defer func() { lookPath = oldLook }()
	lookPath = exec.LookPath
	handle, err := StartVaultAgent(context.Background(), VaultAgentConfig{VaultAddr: "http://vault", SinkPath: filepath.Join(tmp, "token")})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil || handle.ConfigPath == "" || handle.Cmd == nil {
		t.Fatalf("invalid handle")
	}
	_ = handle.Cmd.Wait()
}
