package main

import (
	"net/http"
	"os"
	"testing"

	"carapulse/internal/config"
)

func TestMainPlaceholder(t *testing.T) {
	oldLoad := loadConfig
	loadConfig = func(path string) (config.Config, error) {
		return config.Config{ToolRouter: config.ToolRouterConfig{HTTPAddr: ":8081"}}, nil
	}
	defer func() { loadConfig = oldLoad }()
	if err := run([]string{"-config", "cfg.json"}, func(srv *http.Server) error { return nil }); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestRunListenError(t *testing.T) {
	oldLoad := loadConfig
	loadConfig = func(path string) (config.Config, error) {
		return config.Config{ToolRouter: config.ToolRouterConfig{HTTPAddr: ":8081"}}, nil
	}
	defer func() { loadConfig = oldLoad }()
	err := run([]string{"-config", "cfg.json"}, func(srv *http.Server) error { return http.ErrServerClosed })
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestMainUsesListen(t *testing.T) {
	old := serveHTTP
	serveHTTP = func(srv *http.Server) error { return nil }
	defer func() { serveHTTP = old }()
	oldArgs := os.Args
	os.Args = []string{"tool-router", "-config", "cfg.json"}
	defer func() { os.Args = oldArgs }()
	oldLoad := loadConfig
	loadConfig = func(path string) (config.Config, error) {
		return config.Config{ToolRouter: config.ToolRouterConfig{HTTPAddr: ":8081"}}, nil
	}
	defer func() { loadConfig = oldLoad }()
	main()
}

func TestMainFatalOnError(t *testing.T) {
	oldFatal := fatalf
	called := false
	fatalf = func(format string, args ...any) { called = true }
	defer func() { fatalf = oldFatal }()

	oldServe := serveHTTP
	serveHTTP = func(srv *http.Server) error { return http.ErrServerClosed }
	defer func() { serveHTTP = oldServe }()
	oldArgs := os.Args
	os.Args = []string{"tool-router", "-config", "cfg.json"}
	defer func() { os.Args = oldArgs }()
	oldLoad := loadConfig
	loadConfig = func(path string) (config.Config, error) {
		return config.Config{ToolRouter: config.ToolRouterConfig{HTTPAddr: ":8081"}}, nil
	}
	defer func() { loadConfig = oldLoad }()

	main()
	if !called {
		t.Fatalf("expected fatal")
	}
}
