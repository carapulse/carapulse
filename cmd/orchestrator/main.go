package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"carapulse/internal/config"
	"carapulse/internal/db"
	"carapulse/internal/logging"
	"carapulse/internal/metrics"
	"carapulse/internal/secrets"
	"carapulse/internal/storage"
	"carapulse/internal/tools"
	"carapulse/internal/workflows"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func main() {
	logging.Init("orchestrator", nil)
	if err := run(os.Args[1:]); err != nil {
		fatalf("orchestrator: %v", err)
	}
}

var fatalf = func(format string, args ...any) {
	slog.Error("fatal", "error", fmt.Sprintf(format, args...))
	os.Exit(1)
}
var loadConfig = config.LoadConfig
var newDB = db.NewDB
var newObjectStore = func(cfg config.ObjectStoreConfig) *storage.ObjectStore {
	return &storage.ObjectStore{Endpoint: cfg.Endpoint, Bucket: cfg.Bucket}
}
var newTemporalClient = func(cfg config.OrchestratorConfig) (client.Client, error) {
	opts := client.Options{HostPort: cfg.TemporalAddr, Namespace: cfg.Namespace}
	return client.Dial(opts)
}

var temporalHealthClient client.Client
var setTemporalHealthClient = func(c client.Client) { temporalHealthClient = c }

type closeFunc func() error

func (c closeFunc) Close() error {
	return c()
}

var newWorker = func(cfg config.OrchestratorConfig) (worker.Worker, io.Closer, error) {
	c, err := newTemporalClient(cfg)
	if err != nil {
		return nil, nil, err
	}
	setTemporalHealthClient(c)
	w := worker.New(c, cfg.TaskQueue, worker.Options{})
	return w, closeFunc(func() error { c.Close(); return nil }), nil
}
var runWorker = func(w worker.Worker) error { return w.Run(worker.InterruptCh()) }
var startWorker = func(rt *workflows.Runtime, store workflows.ExecutionStore, obj *storage.ObjectStore, cfg config.Config) error {
	if cfg.Orchestrator.TemporalAddr == "" {
		return errors.New("orchestrator.temporal_addr required")
	}
	w, closer, err := newWorker(cfg.Orchestrator)
	if err != nil {
		return err
	}
	if closer != nil {
		defer func() { _ = closer.Close() }()
	}
	acts := &workflows.Activities{Store: store, Runtime: rt, Objects: obj}
	w.RegisterWorkflow(workflows.PlanExecutionWorkflow)
	w.RegisterWorkflowWithOptions(workflows.GitOpsDeployWorkflowTemporal, workflow.RegisterOptions{Name: "GitOpsDeployWorkflow"})
	w.RegisterWorkflowWithOptions(workflows.HelmReleaseWorkflowTemporal, workflow.RegisterOptions{Name: "HelmReleaseWorkflow"})
	w.RegisterWorkflowWithOptions(workflows.ScaleServiceWorkflowTemporal, workflow.RegisterOptions{Name: "ScaleServiceWorkflow"})
	w.RegisterWorkflowWithOptions(workflows.IncidentRemediationWorkflowTemporal, workflow.RegisterOptions{Name: "IncidentRemediationWorkflow"})
	w.RegisterWorkflowWithOptions(workflows.SecretRotationWorkflowTemporal, workflow.RegisterOptions{Name: "SecretRotationWorkflow"})
	w.RegisterActivity(acts)
	slog.Info("orchestrator ready", "temporal_addr", cfg.Orchestrator.TemporalAddr)
	return runWorker(w)
}

var startVaultAgent = secrets.StartVaultAgent
var openBoundarySession = func(ctx context.Context, router *tools.Router, sandbox *tools.Sandbox, clients tools.HTTPClients, targetID, duration string, ctxRef tools.ContextRef) (string, error) {
	resp, err := router.Execute(ctx, tools.ExecuteRequest{
		Tool:    "boundary",
		Action:  "session_open",
		Input:   map[string]any{"target_id": targetID, "duration": duration},
		Context: ctxRef,
	}, sandbox, clients)
	if err != nil {
		return "", err
	}
	return secrets.ParseSessionID(resp.Output)
}

const defaultMaxOutputBytes = 1_000_000

func run(args []string) error {
	fs := flag.NewFlagSet("orchestrator", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return errors.New("config required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go func() {
		<-ctx.Done()
		time.AfterFunc(30*time.Second, func() { os.Exit(1) })
	}()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	if cfg.Storage.PostgresDSN == "" {
		return errors.New("storage.postgres_dsn required")
	}
	database, err := newDB(cfg.Storage.PostgresDSN)
	if err != nil {
		return err
	}
	defer database.Close()

	if cfg.Orchestrator.HealthAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})
			mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				ok := true

			pctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if conn := database.Conn(); conn == nil {
				ok = false
			} else if err := conn.PingContext(pctx); err != nil {
				ok = false
			}

				if temporalHealthClient != nil {
					tctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
					defer cancel()
					if _, err := temporalHealthClient.CheckHealth(tctx, nil); err != nil {
						ok = false
					}
				} else if cfg.Orchestrator.TemporalAddr != "" {
					ok = false
				}

			if ok {
				_, _ = w.Write([]byte(`{"status":"ok"}`))
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable"}`))
		})

		healthSrv := &http.Server{Addr: cfg.Orchestrator.HealthAddr, Handler: mux}
		go func() {
			if err := healthSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("health server failed", "error", err)
			}
		}()
		go func() {
			<-ctx.Done()
			sctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = healthSrv.Shutdown(sctx)
		}()
	}

	maxOutput := cfg.Sandbox.MaxOutputBytes
	if maxOutput == 0 {
		maxOutput = defaultMaxOutputBytes
	}
	clients := tools.BuildHTTPClients(cfg, tools.APIConfig{
		PrometheusBase:   cfg.Connectors.Prometheus.Addr,
		AlertmanagerBase: cfg.Connectors.Alertmanager.Addr,
		ThanosBase:       cfg.Connectors.Thanos.Addr,
		GrafanaBase:      cfg.Connectors.Grafana.Addr,
		TempoBase:        cfg.Connectors.Tempo.Addr,
		LinearBase:       cfg.Connectors.Linear.BaseURL,
		PagerDutyBase:    cfg.Connectors.PagerDuty.Addr,
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
		},
	})
	router := tools.NewRouter()
	patterns := cfg.LLM.RedactPatterns
	if len(patterns) == 0 {
		patterns = tools.DefaultRedactPatterns()
	}
	router.Redactor = tools.NewRedactor(patterns)
	sandbox := tools.NewSandboxWithConfig(cfg.Sandbox.Enabled, cfg.Sandbox.Runtime, cfg.Sandbox.Image, cfg.Sandbox.EgressAllowlist, cfg.Sandbox.Mounts)
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
		sessionID, err := openBoundarySession(ctx, router, sandbox, clients, cfg.Connectors.Boundary.TargetID, cfg.Connectors.Boundary.SessionDuration, tools.ContextRef{})
		if err != nil {
			slog.Error("boundary session open failed", "error", err)
		} else {
			if sandbox.Env == nil {
				sandbox.Env = map[string]string{}
			}
			for k, v := range secrets.FormatBoundarySessionEnv(sessionID) {
				sandbox.Env[k] = v
			}
			if cfg.Connectors.Boundary.AutoClose {
				defer func() {
					if err := secrets.CloseBoundarySession(context.Background(), &tools.RouterClient{BaseURL: cfg.ToolRouter.BaseURL, Token: cfg.ToolRouter.AuthToken}, sessionID, tools.ContextRef{}); err != nil {
						slog.Warn("boundary session close failed", "error", err)
					}
				}()
			}
		}
	}
	if cfg.Connectors.Boundary.EnableTunnel && cfg.Connectors.Boundary.TargetID != "" {
		tunnel, err := secrets.StartBoundaryTunnel(ctx, cfg.Connectors.Boundary.TargetID, cfg.Connectors.Boundary.ListenAddr)
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
	rt := workflows.NewRuntime(router, sandbox, clients)
	rt.Redactor = router.Redactor
	return startWorker(rt, database, newObjectStore(cfg.Storage.ObjectStore), cfg)
}
