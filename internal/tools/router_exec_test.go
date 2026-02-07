package tools

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRouterExecuteCLIFirst(t *testing.T) {
	tmp := t.TempDir()
	cli := "kubectl"
	if runtime.GOOS == "windows" {
		cli = "kubectl.bat"
	}
	cliPath := filepath.Join(tmp, cli)
	script := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nexit /b 0\r\n"
	}
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write cli: %v", err)
	}
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })

	router := NewRouter()
	sandbox := &Sandbox{}
	resp, err := router.Execute(context.Background(), ExecuteRequest{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "x", "replicas": 1}}, sandbox, HTTPClients{})
	if err != nil {
		// Environment may not have kubectl or execution may fail; skip strict assertion.
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Used != "cli" {
		t.Fatalf("used: %s", resp.Used)
	}
	if resp.ToolCallID == "" {
		t.Fatalf("missing tool call id")
	}
}

func TestRouterExecuteCLIErrorLogs(t *testing.T) {
	tmp := t.TempDir()
	cli := "kubectl"
	if runtime.GOOS == "windows" {
		cli = "kubectl.bat"
	}
	cliPath := filepath.Join(tmp, cli)
	script := "#!/bin/sh\nexit 1\n"
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nexit /b 1\r\n"
	}
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write cli: %v", err)
	}
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })

	router := NewRouter()
	sandbox := &Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) {
		return []byte("fail"), errors.New("boom")
	}}
	resp, err := router.Execute(context.Background(), ExecuteRequest{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "x", "replicas": 1}}, sandbox, HTTPClients{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if resp.Used != "cli" {
		t.Fatalf("used: %s", resp.Used)
	}
	if resp.ToolCallID == "" {
		t.Fatalf("missing tool call id")
	}
	history := router.Logs.History(resp.ToolCallID)
	if len(history) < 2 {
		t.Fatalf("expected logs")
	}
	if history[len(history)-1].Level != "error" {
		t.Fatalf("expected error log")
	}
}

func TestRouterUnknownTool(t *testing.T) {
	router := NewRouter()
	_, err := router.Execute(context.Background(), ExecuteRequest{Tool: "unknown", Action: "status"}, &Sandbox{}, HTTPClients{})
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestRouterExecuteNoSandbox(t *testing.T) {
	router := NewRouter()
	resp, err := router.Execute(context.Background(), ExecuteRequest{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "svc", "replicas": 1}}, nil, HTTPClients{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if resp.ToolCallID == "" {
		t.Fatalf("missing tool call id")
	}
}

func TestRouterExecuteAPIFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	router := NewRouter()
	clients := HTTPClients{Prometheus: &APIClient{BaseURL: srv.URL}}
	resp, err := router.Execute(context.Background(), ExecuteRequest{Tool: "prometheus", Action: "query", Input: map[string]any{"query": "up"}}, &Sandbox{}, clients)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Used != "api" {
		t.Fatalf("used: %s", resp.Used)
	}
	if resp.ToolCallID == "" {
		t.Fatalf("missing tool call id")
	}
}

func TestRouterExecuteAPINoClient(t *testing.T) {
	router := NewRouter()
	resp, err := router.Execute(context.Background(), ExecuteRequest{Tool: "prometheus", Action: "query", Input: map[string]any{"query": "up"}}, &Sandbox{}, HTTPClients{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if resp.ToolCallID == "" {
		t.Fatalf("missing tool call id")
	}
}

func TestRouterExecuteNoAPISupport(t *testing.T) {
	// Temporarily add tool with no API support.
	orig := Registry
	Registry = append(Registry, Tool{Name: "noapi", CLI: "missing", SupportsAPI: false})
	defer func() { Registry = orig }()
	router := NewRouter()
	resp, err := router.Execute(context.Background(), ExecuteRequest{Tool: "noapi", Action: "status"}, &Sandbox{}, HTTPClients{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if resp.ToolCallID == "" {
		t.Fatalf("missing tool call id")
	}
}

func TestBuildCmdBranches(t *testing.T) {
	cases := []struct {
		tool string
		want string
	}{
		{"kubectl", "kubectl"},
		{"helm", "helm"},
		{"argocd", "argocd"},
		{"aws", "aws"},
		{"vault", "vault"},
		{"boundary", "boundary"},
		{"github", "gh"},
		{"gitlab", "glab"},
		{"custom", "custom"},
	}
	for _, c := range cases {
		cmd := buildCmd(c.tool, "status", map[string]any{})
		if len(cmd) == 0 || cmd[0] != c.want {
			t.Fatalf("%s: %v", c.tool, cmd)
		}
	}
}

func TestRouterExecuteHelmValuesRef(t *testing.T) {
	tmp := t.TempDir()
	cli := "helm"
	script := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		cli = "helm.bat"
		script = "@echo off\r\nexit /b 0\r\n"
	}
	cliPath := filepath.Join(tmp, cli)
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write cli: %v", err)
	}
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })

	oldResolve := resolveArtifact
	oldCreate := createTempFile
	oldRemove := removeFile
	resolveArtifact = func(ref ArtifactRef) ([]byte, error) { return []byte("values"), nil }
	createTempFile = func(dir, pattern string) (*os.File, error) {
		return os.CreateTemp(t.TempDir(), pattern)
	}
	var removed string
	removeFile = func(path string) error {
		removed = path
		return os.Remove(path)
	}
	t.Cleanup(func() {
		resolveArtifact = oldResolve
		createTempFile = oldCreate
		removeFile = oldRemove
	})

	var valuesPath string
	sandbox := &Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) {
		for i := 0; i < len(cmd)-1; i++ {
			if cmd[i] == "-f" {
				valuesPath = cmd[i+1]
			}
		}
		return []byte("ok"), nil
	}}
	router := NewRouter()
	resp, err := router.Execute(context.Background(), ExecuteRequest{
		Tool:   "helm",
		Action: "upgrade",
		Input: map[string]any{
			"release":    "rel",
			"chart":      "chart",
			"values_ref": map[string]any{"kind": "inline", "ref": "data"},
		},
	}, sandbox, HTTPClients{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Used != "cli" {
		t.Fatalf("used: %s", resp.Used)
	}
	if valuesPath == "" {
		t.Fatalf("missing values path")
	}
	if removed == "" {
		t.Fatalf("expected cleanup")
	}
}

func TestRouterExecuteHelmValuesRefInvalid(t *testing.T) {
	tmp := t.TempDir()
	cli := "helm"
	script := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		cli = "helm.bat"
		script = "@echo off\r\nexit /b 0\r\n"
	}
	cliPath := filepath.Join(tmp, cli)
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write cli: %v", err)
	}
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })

	router := NewRouter()
	_, err := router.Execute(context.Background(), ExecuteRequest{
		Tool:   "helm",
		Action: "upgrade",
		Input: map[string]any{
			"release":    "rel",
			"values_ref": map[string]any{"kind": "bad", "ref": "data"},
		},
	}, &Sandbox{}, HTTPClients{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRouterExecuteHelmValuesRefResolveError(t *testing.T) {
	tmp := t.TempDir()
	cli := "helm"
	script := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		cli = "helm.bat"
		script = "@echo off\r\nexit /b 0\r\n"
	}
	cliPath := filepath.Join(tmp, cli)
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write cli: %v", err)
	}
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })

	oldResolve := resolveArtifact
	resolveArtifact = func(ref ArtifactRef) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { resolveArtifact = oldResolve })

	router := NewRouter()
	_, err := router.Execute(context.Background(), ExecuteRequest{
		Tool:   "helm",
		Action: "upgrade",
		Input: map[string]any{
			"release":    "rel",
			"values_ref": map[string]any{"kind": "inline", "ref": "data"},
		},
	}, &Sandbox{}, HTTPClients{})
	if err == nil {
		t.Fatalf("expected error")
	}
}
