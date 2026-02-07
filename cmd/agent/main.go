package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"carapulse/internal/chatops"
	"carapulse/internal/config"
	"carapulse/internal/logging"
	"carapulse/internal/metrics"
)

const defaultSlackPath = "/slack/commands"

func main() {
	logging.Init("agent", nil)
	if err := run(os.Args[1:], serveHTTP); err != nil {
		fatalf("agent: %v", err)
	}
}

var serveHTTP = func(srv *http.Server) error { return srv.ListenAndServe() }
var fatalf = func(format string, args ...any) {
	slog.Error("fatal", "error", fmt.Sprintf(format, args...))
	os.Exit(1)
}
var loadConfig = config.LoadConfig
var newSlackHandler = chatops.NewSlackHandler
var newGatewayClient = func(baseURL, token string) chatops.GatewayClient {
	return &chatops.HTTPGatewayClient{BaseURL: baseURL, Token: token}
}

func run(args []string, serve func(*http.Server) error) error {
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config JSON")
	addr := fs.String("addr", ":8090", "listen address")
	path := fs.String("path", defaultSlackPath, "slack endpoint path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return errors.New("config required")
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	if cfg.ChatOps.SlackSigningSecret == "" {
		return errors.New("slack signing secret required")
	}
	gatewayURL := normalizeGatewayURL(cfg.ChatOps.GatewayURL, cfg.Gateway.HTTPAddr)
	if gatewayURL == "" {
		return errors.New("gateway url required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	client := newGatewayClient(gatewayURL, cfg.ChatOps.GatewayToken)
	handler := newSlackHandler(cfg.ChatOps.SlackSigningSecret, client)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		c := &http.Client{Timeout: 2 * time.Second}
		resp, err := c.Get(strings.TrimRight(gatewayURL, "/") + "/healthz")
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable"}`))
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/metrics", metrics.Handler())
	mux.Handle(normalizePath(*path), handler)
	slog.Info("agent listening", "addr", *addr)
	httpSrv := &http.Server{Addr: *addr, Handler: mux}
	errCh := make(chan error, 1)
	go func() { errCh <- serve(httpSrv) }()
	select {
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	forceExit := time.AfterFunc(30*time.Second, func() { os.Exit(1) })
	defer forceExit.Stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	err = <-errCh
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func normalizeGatewayURL(gatewayURL, addr string) string {
	if gatewayURL != "" {
		return strings.TrimRight(gatewayURL, "/")
	}
	if addr == "" {
		return ""
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return strings.TrimRight(addr, "/")
	}
	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr
	}
	return "http://" + addr
}

func normalizePath(path string) string {
	if path == "" {
		return defaultSlackPath
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}
