package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"carapulse/internal/config"
	"carapulse/internal/logging"
	"carapulse/internal/metrics"
	"carapulse/internal/policy"
	"carapulse/internal/secrets"
	"carapulse/internal/tools"
)

func main() {
	logging.Init("tool-router", nil)
	if err := run(os.Args[1:], serveHTTP); err != nil {
		fatalf("tool-router: %v", err)
	}
}

var serveHTTP = func(srv *http.Server) error { return srv.ListenAndServe() }
var fatalf = func(format string, args ...any) {
	slog.Error("fatal", "error", fmt.Sprintf(format, args...))
	os.Exit(1)
}
var loadConfig = config.LoadConfig
var newPolicyService = func(cfg config.PolicyConfig) *policy.PolicyService {
	return &policy.PolicyService{OPAURL: cfg.OPAURL, PolicyPackage: cfg.PolicyPackage}
}
var startVaultAgent = secrets.StartVaultAgent

const defaultMaxOutputBytes = 1_000_000

func run(args []string, serve func(*http.Server) error) error {
	fs := flag.NewFlagSet("tool-router", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return errors.New("config required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	tools.SetWorkspaceDir(cfg.Storage.WorkspaceDir)
	addr := cfg.ToolRouter.HTTPAddr
	if addr == "" {
		addr = ":8081"
	}
	router := tools.NewRouter()
	patterns := cfg.LLM.RedactPatterns
	if len(patterns) == 0 {
		patterns = tools.DefaultRedactPatterns()
	}
	router.Redactor = tools.NewRedactor(patterns)
	sandbox := tools.NewSandboxWithConfig(cfg.Sandbox.Enabled, cfg.Sandbox.Runtime, cfg.Sandbox.Image, cfg.Sandbox.EgressAllowlist, cfg.Sandbox.Mounts)
	maxOutput := cfg.Sandbox.MaxOutputBytes
	if maxOutput == 0 {
		maxOutput = defaultMaxOutputBytes
	}
	sandbox.Enforce = cfg.Sandbox.Enforce
	sandbox.ReadOnlyRoot = cfg.Sandbox.ReadOnlyRoot
	sandbox.Tmpfs = cfg.Sandbox.Tmpfs
	sandbox.User = cfg.Sandbox.User
	sandbox.SeccompProfile = cfg.Sandbox.SeccompProfile
	sandbox.NoNewPrivs = cfg.Sandbox.NoNewPrivs
	sandbox.DropCaps = cfg.Sandbox.DropCaps
	sandbox.RequireSeccomp = cfg.Sandbox.RequireSeccomp
	sandbox.RequireNoNewPrivs = cfg.Sandbox.RequireNoNewPrivs
	sandbox.RequireUser = cfg.Sandbox.RequireUser
	sandbox.RequireDropCaps = cfg.Sandbox.RequireDropCaps
	sandbox.RequireOnWrite = cfg.Sandbox.RequireOnWrite
	sandbox.RequireEgressAllowlist = cfg.Sandbox.RequireEgressAllowlist
	sandbox.MaxOutputBytes = maxOutput
	clients := tools.BuildHTTPClients(cfg, tools.APIConfig{
		PrometheusBase:   cfg.Connectors.Prometheus.Addr,
		AlertmanagerBase: cfg.Connectors.Alertmanager.Addr,
		ThanosBase:       cfg.Connectors.Thanos.Addr,
		GrafanaBase:      cfg.Connectors.Grafana.Addr,
		TempoBase:        cfg.Connectors.Tempo.Addr,
		LinearBase:       cfg.Connectors.Linear.BaseURL,
		PagerDutyBase:    cfg.Connectors.PagerDuty.Addr,
		VaultBase:        cfg.Connectors.Vault.Addr,
		BoundaryBase:     cfg.Connectors.Boundary.Addr,
		ArgoCDBase:       cfg.Connectors.ArgoCD.Addr,
		AWSBase:          cfg.Connectors.AWS.Addr,
		GrafanaOrgID:     cfg.Connectors.Grafana.OrgID,
		EgressAllowlist:  cfg.Sandbox.EgressAllowlist,
		MaxOutputBytes:   maxOutput,
		Tokens: map[string]string{
			"prometheus":   cfg.Connectors.Prometheus.Token,
			"alertmanager": cfg.Connectors.Alertmanager.Token,
			"thanos":       cfg.Connectors.Thanos.Token,
			"grafana":      cfg.Connectors.Grafana.Token,
			"tempo":        cfg.Connectors.Tempo.Token,
			"linear":       cfg.Connectors.Linear.Token,
			"pagerduty":    cfg.Connectors.PagerDuty.Token,
			"vault":        cfg.Connectors.Vault.Token,
			"boundary":     cfg.Connectors.Boundary.Token,
			"argocd":       cfg.Connectors.ArgoCD.Token,
			"aws":          cfg.Connectors.AWS.Token,
		},
	})
	if cfg.Connectors.Vault.Addr != "" && cfg.Connectors.Vault.SinkPath != "" {
		templateSource, templateDest := secrets.ResolveTemplatePaths(cfg.Connectors.Vault.TemplateDir, cfg.Connectors.Vault.TemplateSource, cfg.Connectors.Vault.TemplateDest)
		agentCfg, err := secrets.BuildVaultAgentConfigFromConnectors(
			cfg.Connectors.Vault.Addr,
			cfg.Connectors.Vault.Namespace,
			cfg.Connectors.Vault.AutoAuthMethod,
			cfg.Connectors.Vault.AgentRole,
			cfg.Connectors.Vault.AuthPath,
			cfg.Connectors.Vault.AppRoleID,
			cfg.Connectors.Vault.AppRoleSecret,
			cfg.Connectors.Vault.SinkPath,
			templateSource,
			templateDest,
			cfg.Connectors.Vault.RetryMaxBackoff,
		)
		if err != nil {
			slog.Error("vault agent config failed", "error", err)
		} else if _, err := startVaultAgent(ctx, agentCfg); err != nil {
			slog.Error("vault agent start failed", "error", err)
		} else {
			slog.Info("vault agent started", "config", secrets.DescribeVaultAgent(agentCfg))
			if env, err := secrets.LoadTemplateEnv(templateDest); err == nil {
				if sandbox.Env == nil {
					sandbox.Env = map[string]string{}
				}
				for k, v := range env {
					sandbox.Env[k] = v
				}
			}
		}
	}
	if cfg.Connectors.Vault.AuditEnable {
		input := map[string]any{
			"type": "file",
			"path": cfg.Connectors.Vault.AuditPath,
		}
		if _, err := router.Execute(ctx, tools.ExecuteRequest{Tool: "vault", Action: "audit_enable", Input: input}, sandbox, clients); err != nil {
			slog.Error("vault audit enable failed", "error", err)
		}
	}
	if cfg.Connectors.Vault.HealthIntervalSecs > 0 {
		interval := time.Duration(cfg.Connectors.Vault.HealthIntervalSecs) * time.Second
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if _, err := router.Execute(ctx, tools.ExecuteRequest{Tool: "vault", Action: "health", Input: map[string]any{}}, sandbox, clients); err != nil {
						slog.Warn("vault health check failed", "error", err)
					}
				}
			}
		}()
	}
	if cfg.Connectors.Vault.RenewIntervalSecs > 0 {
		interval := time.Duration(cfg.Connectors.Vault.RenewIntervalSecs) * time.Second
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if _, err := router.Execute(ctx, tools.ExecuteRequest{Tool: "vault", Action: "token_renew", Input: map[string]any{}}, sandbox, clients); err != nil {
						slog.Warn("vault token renew failed", "error", err)
					}
				}
			}
		}()
	}
	if cfg.Connectors.Boundary.TargetID != "" {
		resp, err := router.Execute(ctx, tools.ExecuteRequest{
			Tool:   "boundary",
			Action: "session_open",
			Input: map[string]any{
				"target_id": cfg.Connectors.Boundary.TargetID,
				"duration":  cfg.Connectors.Boundary.SessionDuration,
			},
		}, sandbox, clients)
		if err != nil {
			slog.Error("boundary session open failed", "error", err)
		} else if sessionID, err := secrets.ParseSessionID(resp.Output); err != nil {
			slog.Error("boundary session parse failed", "error", err)
		} else {
			if sandbox.Env == nil {
				sandbox.Env = map[string]string{}
			}
			for k, v := range secrets.FormatBoundarySessionEnv(sessionID) {
				sandbox.Env[k] = v
			}
			if cfg.Connectors.Boundary.AutoClose {
				defer func() {
					if _, err := router.Execute(context.Background(), tools.ExecuteRequest{Tool: "boundary", Action: "session_close", Input: map[string]any{"session_id": sessionID}}, sandbox, clients); err != nil {
						slog.Warn("boundary session close failed", "error", err)
					}
				}()
			}
		}
	}
	if cfg.Connectors.Boundary.EnableTunnel && cfg.Connectors.Boundary.TargetID != "" {
		tunnel, err := secrets.StartBoundaryTunnel(context.Background(), cfg.Connectors.Boundary.TargetID, cfg.Connectors.Boundary.ListenAddr)
		if err != nil {
			slog.Error("boundary tunnel start failed", "error", err)
		} else {
			defer func() {
				if err := secrets.StopBoundaryTunnel(tunnel); err != nil {
					slog.Warn("boundary tunnel close failed", "error", err)
				}
			}()
		}
	}
	server := tools.NewServer(router, sandbox, clients)
	server.Auth = tools.AuthConfig{
		Token:    cfg.ToolRouter.AuthToken,
		Issuer:   cfg.ToolRouter.OIDCIssuer,
		Audience: cfg.ToolRouter.OIDCClientID,
		JWKSURL:  cfg.ToolRouter.OIDCJWKSURL,
	}
	if cfg.Policy.OPAURL != "" {
		server.Policy = &policy.Evaluator{Checker: newPolicyService(cfg.Policy)}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		checks := map[string]string{}
		ok := true

		if cfg.Sandbox.Enabled {
			rt := strings.TrimSpace(cfg.Sandbox.Runtime)
			if rt == "" {
				rt = "docker"
			}
			cctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			cmd := exec.CommandContext(cctx, rt, "version")
			if err := cmd.Run(); err != nil {
				ok = false
				checks["sandbox_runtime"] = err.Error()
			} else {
				checks["sandbox_runtime"] = "ok"
			}
		}

		if cfg.Policy.OPAURL != "" {
			opaURL := strings.TrimRight(cfg.Policy.OPAURL, "/") + "/health"
			c := &http.Client{Timeout: 2 * time.Second}
			resp, err := c.Get(opaURL)
			if err != nil {
				ok = false
				checks["opa"] = err.Error()
			} else {
				_ = resp.Body.Close()
				if resp.StatusCode/100 != 2 {
					ok = false
					checks["opa"] = resp.Status
				} else {
					checks["opa"] = "ok"
				}
			}
		}

		if ok {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		if data, err := json.Marshal(map[string]any{"status": "unavailable", "checks": checks}); err == nil {
			_, _ = w.Write(data)
			return
		}
		_, _ = w.Write([]byte(`{"status":"unavailable"}`))
	})
	mux.Handle("/metrics", metrics.Handler())
	mux.Handle("/", server)

	httpSrv := &http.Server{Addr: addr, Handler: metrics.Middleware(mux)}
	errCh := make(chan error, 1)
	go func() { errCh <- serve(httpSrv) }()

	slog.Info("tool-router listening", "addr", addr)
	select {
	case err := <-errCh:
		if err == nil {
			return nil
		}
		if errors.Is(err, http.ErrServerClosed) && ctx.Err() != nil {
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
