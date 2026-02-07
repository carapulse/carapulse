package workflows

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"carapulse/internal/tools"
)

func TestActivitiesAPICalls(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	handler.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	handler.HandleFunc("/api/annotations", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	handler.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"id":"issue"}}`))
	})
	handler.HandleFunc("/v2/enqueue", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"incident"}`))
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	rt := NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{
		Prometheus: &tools.APIClient{BaseURL: server.URL},
		Tempo:      &tools.APIClient{BaseURL: server.URL},
		Grafana:    &tools.APIClient{BaseURL: server.URL},
		Linear:     &tools.APIClient{BaseURL: server.URL},
		PagerDuty:  &tools.APIClient{BaseURL: server.URL},
	})
	ctx := context.Background()
	if _, err := QueryPrometheusActivity(ctx, "up", rt); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := QueryTempoActivity(ctx, "trace", rt); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := CreateGrafanaAnnotationActivity(ctx, []byte(`{"text":"note"}`), rt); err != nil {
		t.Fatalf("err: %v", err)
	}
	if out, err := CreateLinearIssueActivity(ctx, []byte(`{"title":"issue"}`), rt); err != nil || out == "" {
		t.Fatalf("err: %v out=%s", err, out)
	}
	if out, err := CreatePagerDutyIncidentActivity(ctx, []byte(`{"incident":"x"}`), rt); err != nil || out == "" {
		t.Fatalf("err: %v out=%s", err, out)
	}
}

func TestActivitiesErrors(t *testing.T) {
	if _, err := QueryPrometheusActivity(context.Background(), "up", nil); err == nil {
		t.Fatalf("expected error")
	}
	if err := CreateGrafanaAnnotationActivity(context.Background(), []byte("{"), NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := CreateLinearIssueActivity(context.Background(), nil, nil); err == nil {
		t.Fatalf("expected error")
	}
}

func writeCLIScript(t *testing.T, dir, name, script, scriptWin string) {
	if runtime.GOOS == "windows" {
		name += ".bat"
		script = "@echo off\r\n" + scriptWin + "\r\n"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestHelmStatusActivity(t *testing.T) {
	tmp := t.TempDir()
	writeCLIScript(t, tmp, "helm", "#!/bin/sh\necho ok\nexit 0\n", "echo ok\r\nexit /b 0")
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() { _ = os.Setenv("PATH", oldPath) }()

	rt := NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})
	out, err := HelmStatusActivity(context.Background(), "rel", rt)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) == "" {
		t.Fatalf("expected output")
	}
}

func TestCreateGitPullRequestActivityCLI(t *testing.T) {
	tmp := t.TempDir()
	writeCLIScript(t, tmp, "gh", "#!/bin/sh\necho ok\nexit 0\n", "echo ok\r\nexit /b 0")
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() { _ = os.Setenv("PATH", oldPath) }()

	rt := NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})
	out, err := CreateGitPullRequestActivity(context.Background(), []byte("{}"), rt)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == "" {
		t.Fatalf("expected output")
	}
}

func TestDecodePayloadPassthrough(t *testing.T) {
	input := map[string]any{"key": "value"}
	out, err := decodePayload(input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	encoded, _ := json.Marshal(out)
	if string(encoded) == "" {
		t.Fatalf("expected output")
	}
}

func TestDecodePayloadEmptyBytes(t *testing.T) {
	out, err := decodePayload([]byte{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil")
	}
}

func TestDecodePayloadNil(t *testing.T) {
	out, err := decodePayload(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil")
	}
}

func TestDecodePayloadBadJSON(t *testing.T) {
	if _, err := decodePayload([]byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunToolMissingRuntime(t *testing.T) {
	if _, err := runTool(context.Background(), nil, "prometheus", "query", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunToolDecodeError(t *testing.T) {
	rt := NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})
	_, err := runTool(context.Background(), rt, "prometheus", "query", []byte("{"))
	if err == nil {
		t.Fatalf("expected error")
	}
}
