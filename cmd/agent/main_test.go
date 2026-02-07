package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"carapulse/internal/chatops"
	"carapulse/internal/config"
)

func TestRunMissingConfig(t *testing.T) {
	if err := run([]string{}, func(srv *http.Server) error { return nil }); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunBadConfig(t *testing.T) {
	if err := run([]string{"-config", "/nope.json"}, func(srv *http.Server) error { return nil }); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunBadFlag(t *testing.T) {
	if err := run([]string{"-badflag"}, func(srv *http.Server) error { return nil }); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunMissingSlackSecret(t *testing.T) {
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":8080"},"policy":{"opa_url":"http://opa","policy_package":"p"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}},"chatops":{"gateway_url":"http://gateway"}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := run([]string{"-config", file}, func(srv *http.Server) error { return nil }); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunMissingGatewayURL(t *testing.T) {
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":""},"policy":{"opa_url":"http://opa","policy_package":"p"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}},"chatops":{"slack_signing_secret":"secret"}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := run([]string{"-config", file}, func(srv *http.Server) error { return nil }); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunGatewayURLRequired(t *testing.T) {
	oldLoad := loadConfig
	loadConfig = func(path string) (config.Config, error) {
		return config.Config{ChatOps: config.ChatOpsConfig{SlackSigningSecret: "secret"}}, nil
	}
	defer func() { loadConfig = oldLoad }()

	if err := run([]string{"-config", "cfg.json"}, func(srv *http.Server) error { return nil }); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunOK(t *testing.T) {
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":8080"},"policy":{"opa_url":"http://opa","policy_package":"p"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}},"chatops":{"slack_signing_secret":"secret","gateway_token":"tok"}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := run([]string{"-config", file, "-addr", ":9090", "-path", "slack"}, func(srv *http.Server) error {
		if srv.Addr != ":9090" {
			t.Fatalf("addr: %s", srv.Addr)
		}
		mux, ok := srv.Handler.(*http.ServeMux)
		if !ok {
			t.Fatalf("handler: %T", srv.Handler)
		}
		req := httptest.NewRequest(http.MethodPost, "/slack", nil)
		h, pattern := mux.Handler(req)
		if pattern == "" {
			t.Fatalf("missing route")
		}
		slack, ok := h.(*chatops.SlackHandler)
		if !ok {
			t.Fatalf("handler: %T", h)
		}
		if slack.SigningSecret != "secret" {
			t.Fatalf("secret: %s", slack.SigningSecret)
		}
		client, ok := slack.Client.(*chatops.HTTPGatewayClient)
		if !ok {
			t.Fatalf("client: %T", slack.Client)
		}
		if client.BaseURL != "http://127.0.0.1:8080" || client.Token != "tok" {
			t.Fatalf("client: %#v", client)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestRunGatewayURLOverride(t *testing.T) {
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":8080"},"policy":{"opa_url":"http://opa","policy_package":"p"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}},"chatops":{"slack_signing_secret":"secret","gateway_url":"https://example.com/api/"}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := run([]string{"-config", file, "-path", "/slack/commands"}, func(srv *http.Server) error {
		mux := srv.Handler.(*http.ServeMux)
		req := httptest.NewRequest(http.MethodPost, "/slack/commands", nil)
		h, pattern := mux.Handler(req)
		if pattern == "" {
			t.Fatalf("missing route")
		}
		slack := h.(*chatops.SlackHandler)
		client := slack.Client.(*chatops.HTTPGatewayClient)
		if client.BaseURL != "https://example.com/api" {
			t.Fatalf("base: %s", client.BaseURL)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestNormalizeGatewayURL(t *testing.T) {
	if got := normalizeGatewayURL("http://example.com/", ""); got != "http://example.com" {
		t.Fatalf("got: %s", got)
	}
	if got := normalizeGatewayURL("", ":8080"); got != "http://127.0.0.1:8080" {
		t.Fatalf("got: %s", got)
	}
	if got := normalizeGatewayURL("", "127.0.0.1:8080"); got != "http://127.0.0.1:8080" {
		t.Fatalf("got: %s", got)
	}
	if got := normalizeGatewayURL("", "https://example.com/"); got != "https://example.com" {
		t.Fatalf("got: %s", got)
	}
	if got := normalizeGatewayURL("", ""); got != "" {
		t.Fatalf("got: %s", got)
	}
}

func TestNormalizePath(t *testing.T) {
	if got := normalizePath(""); got != defaultSlackPath {
		t.Fatalf("got: %s", got)
	}
	if got := normalizePath("/x"); got != "/x" {
		t.Fatalf("got: %s", got)
	}
	if got := normalizePath("x"); got != "/x" {
		t.Fatalf("got: %s", got)
	}
}

func TestMainFatalOnError(t *testing.T) {
	oldFatal := fatalf
	called := false
	fatalf = func(format string, args ...any) { called = true }
	defer func() { fatalf = oldFatal }()

	oldArgs := os.Args
	os.Args = []string{"agent"}
	defer func() { os.Args = oldArgs }()

	main()
	if !called {
		t.Fatalf("expected fatal")
	}
}
