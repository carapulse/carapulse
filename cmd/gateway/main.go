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
	"os/signal"
	"sync"
	"syscall"
	"time"

	"carapulse/internal/approvals"
	"carapulse/internal/config"
	ctxmodel "carapulse/internal/context"
	"carapulse/internal/context/collectors"
	"carapulse/internal/db"
	"carapulse/internal/llm"
	"carapulse/internal/logging"
	"carapulse/internal/metrics"
	"carapulse/internal/policy"
	"carapulse/internal/storage"
	"carapulse/internal/tools"
	"carapulse/internal/web"
	"carapulse/internal/workflows"
	"go.temporal.io/sdk/client"
)

const defaultPolicyPackage = "policy.assistant.v1"

func main() {
	logging.Init("gateway", nil)
	if err := run(os.Args[1:], serveHTTP); err != nil {
		fatalf("gateway: %v", err)
	}
}

var serveHTTP = func(srv *http.Server) error { return srv.ListenAndServe() }
var fatalf = func(format string, args ...any) {
	slog.Error("fatal", "error", fmt.Sprintf(format, args...))
	os.Exit(1)
}
var newDB = db.NewDB
var newServer = web.NewServer
var newLinearClient = approvals.NewLinearClient
var newLLMRouter = func(cfg config.LLMConfig) *llm.Router {
	router := &llm.Router{
		Provider:       cfg.Provider,
		APIKey:         cfg.APIKey,
		Model:          cfg.Model,
		APIBase:        cfg.APIBase,
		MaxTokens:      cfg.MaxOutputTokens,
		AuthProfile:    cfg.AuthProfile,
		AuthPath:       cfg.AuthPath,
		RedactPatterns: cfg.RedactPatterns,
	}
	if cfg.TimeoutMS > 0 {
		router.HTTPClient = &http.Client{Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond}
	}
	return router
}
var newPolicyService = func(cfg config.PolicyConfig) *policy.PolicyService {
	pkg := cfg.PolicyPackage
	if pkg == "" {
		pkg = defaultPolicyPackage
	}
	return &policy.PolicyService{OPAURL: cfg.OPAURL, PolicyPackage: pkg}
}
var newTemporalClient = func(cfg config.OrchestratorConfig) (client.Client, error) {
	opts := client.Options{HostPort: cfg.TemporalAddr, Namespace: cfg.Namespace}
	return client.Dial(opts)
}
var newObjectStore = func(cfg config.ObjectStoreConfig) *storage.ObjectStore {
	return &storage.ObjectStore{Endpoint: cfg.Endpoint, Bucket: cfg.Bucket}
}
var newToolRouterClient = func(cfg config.ToolRouterConfig) *tools.RouterClient {
	return &tools.RouterClient{BaseURL: cfg.BaseURL, Token: cfg.AuthToken}
}
var newContextService = func(cfg config.Config, store ctxmodel.Store, router *tools.RouterClient) *ctxmodel.ContextService {
	if store == nil || router == nil || !cfg.Context.Enabled {
		return nil
	}
	labels := map[string]string{}
	for k, v := range cfg.Context.Labels {
		labels[k] = v
	}
	if labels["tenant_id"] == "" && cfg.Context.TenantID != "" {
		labels["tenant_id"] = cfg.Context.TenantID
	}
	if labels["environment"] == "" && cfg.Context.Environment != "" {
		labels["environment"] = cfg.Context.Environment
	}
	if labels["cluster_id"] == "" && cfg.Context.ClusterID != "" {
		labels["cluster_id"] = cfg.Context.ClusterID
	}
	if labels["namespace"] == "" && cfg.Context.Namespace != "" {
		labels["namespace"] = cfg.Context.Namespace
	}
	if labels["aws_account_id"] == "" && cfg.Context.AWSAccountID != "" {
		labels["aws_account_id"] = cfg.Context.AWSAccountID
	}
	if labels["region"] == "" && cfg.Context.Region != "" {
		labels["region"] = cfg.Context.Region
	}
	if labels["argocd_project"] == "" && cfg.Context.ArgoCDProject != "" {
		labels["argocd_project"] = cfg.Context.ArgoCDProject
	}
	if labels["grafana_org_id"] == "" && cfg.Context.GrafanaOrgID != "" {
		labels["grafana_org_id"] = cfg.Context.GrafanaOrgID
	}
	ctxRef := tools.ContextRef{
		TenantID:      cfg.Context.TenantID,
		Environment:   cfg.Context.Environment,
		ClusterID:     cfg.Context.ClusterID,
		Namespace:     cfg.Context.Namespace,
		AWSAccountID:  cfg.Context.AWSAccountID,
		Region:        cfg.Context.Region,
		ArgoCDProject: cfg.Context.ArgoCDProject,
		GrafanaOrgID:  cfg.Context.GrafanaOrgID,
	}
	base := collectors.Base{Router: router, Context: ctxRef, Labels: labels}
	var pollers []ctxmodel.Poller
	var watchers []ctxmodel.Watcher

	argo := &collectors.ArgoCDPoller{Base: base, Apps: cfg.Context.ArgoApps}
	pollers = append(pollers, argo)

	awsTags := []map[string]any{}
	for key, values := range cfg.Context.AWSResourceTags {
		if len(values) == 0 {
			continue
		}
		awsTags = append(awsTags, map[string]any{"key": key, "values": values})
	}
	aws := &collectors.AWSPoller{Base: base, TagFilters: awsTags, Region: cfg.Context.Region}
	pollers = append(pollers, aws)

	prom := &collectors.PromPoller{Base: base, Queries: cfg.Context.PromQLQueries}
	pollers = append(pollers, prom)
	if cfg.Connectors.Thanos.Addr != "" {
		pollers = append(pollers, &collectors.ThanosPoller{Base: base, Queries: cfg.Context.PromQLQueries})
	}

	if cfg.Connectors.Alertmanager.Addr != "" {
		pollers = append(pollers, &collectors.AlertmanagerPoller{Base: base})
	}

	tempo := &collectors.TempoPoller{Base: base, Queries: cfg.Context.TraceQLQueries}
	pollers = append(pollers, tempo)

	graf := &collectors.GrafanaPoller{Base: base, FolderIDs: cfg.Context.GrafanaFolders}
	pollers = append(pollers, graf)

	if len(cfg.Context.Services) > 0 {
		var mappings []collectors.ServiceMapping
		for _, svc := range cfg.Context.Services {
			mappings = append(mappings, collectors.ServiceMapping{
				Service:     svc.Service,
				Environment: svc.Environment,
				ClusterID:   svc.ClusterID,
				Namespace:   svc.Namespace,
				PromQL:      svc.PromQL,
				TraceQL:     svc.TraceQL,
				Dashboards:  svc.Dashboards,
			})
		}
		pollers = append(pollers, &collectors.StaticMappingPoller{Base: base, Mappings: mappings})
	}
	if extra, err := collectors.LoadServiceMappings(cfg.Storage.WorkspaceDir); err == nil && len(extra) > 0 {
		pollers = append(pollers, &collectors.StaticMappingPoller{Base: base, Mappings: extra})
	}

	helmNamespaces := cfg.Context.HelmNamespaces
	if len(helmNamespaces) == 0 && cfg.Context.Namespace != "" {
		helmNamespaces = []string{cfg.Context.Namespace}
	}
	helm := &collectors.HelmPoller{Base: base, Namespaces: helmNamespaces}
	pollers = append(pollers, helm)

	k8sNamespaces := cfg.Context.K8sNamespaces
	if len(k8sNamespaces) == 0 && cfg.Context.Namespace != "" {
		k8sNamespaces = []string{cfg.Context.Namespace}
	}
	if len(k8sNamespaces) == 0 {
		k8sNamespaces = []string{""}
	}
	k8sResources := cfg.Context.K8sResources
	if len(k8sResources) == 0 {
		k8sResources = collectors.DefaultK8sResources()
	}
	for _, ns := range k8sNamespaces {
		sendInitial := cfg.Context.WatchSendInitialEvents
		allowBookmarks := cfg.Context.WatchAllowBookmarks
		if !cfg.Context.WatchSendInitialEvents {
			sendInitial = true
		}
		if !cfg.Context.WatchAllowBookmarks {
			allowBookmarks = true
		}
		pollers = append(pollers, &collectors.K8sPoller{Base: base, Resources: k8sResources, Namespace: ns})
		watchers = append(watchers, &collectors.K8sWatcher{
			Base:              base,
			Resources:         k8sResources,
			Namespace:         ns,
			Timeout:           time.Duration(cfg.Context.WatchTimeoutSecs) * time.Second,
			SendInitialEvents: sendInitial,
			AllowBookmarks:    allowBookmarks,
		})
	}
	if cfg.Context.WatchIntervalSecs > 0 {
		interval := time.Duration(cfg.Context.WatchIntervalSecs) * time.Second
		for _, poller := range pollers {
			if poller == nil {
				continue
			}
			if _, ok := poller.(*collectors.K8sPoller); ok {
				continue
			}
			watchers = append(watchers, &collectors.PollingWatcher{Poller: poller, Interval: interval})
		}
	}
	svc := ctxmodel.NewWithStore(store)
	svc.Pollers = pollers
	svc.Watchers = watchers
	if cfg.Context.PollIntervalSecs > 0 {
		svc.PollInterval = time.Duration(cfg.Context.PollIntervalSecs) * time.Second
	}
	if cfg.Context.SnapshotIntervalSecs > 0 {
		svc.SnapshotInterval = time.Duration(cfg.Context.SnapshotIntervalSecs) * time.Second
	}
	return svc
}

var startScheduler = func(ctx context.Context, wg *sync.WaitGroup, gt *web.GoroutineTracker, s *web.Scheduler) {
	if s == nil {
		return
	}
	if gt == nil {
		gt = web.NewGoroutineTracker()
	}
	gt.Go(ctx, wg, "scheduler", func(ctx context.Context) error { return s.Run(ctx) })
}

var approvalRunContext = func(ctx context.Context) context.Context { return ctx }
var approvalRun = func(ctx context.Context, w *approvals.Watcher) error { return w.Run(ctx) }
var startApprovalWatcher = func(ctx context.Context, wg *sync.WaitGroup, gt *web.GoroutineTracker, client approvals.ApprovalClient, store approvals.ApprovalStore, cfg config.LinearConfig) {
	watcher := approvals.NewWatcher(client, store)
	if cfg.PollIntervalMS > 0 {
		watcher.PollInterval = time.Duration(cfg.PollIntervalMS) * time.Millisecond
	}
	if cfg.TimeoutHours > 0 {
		watcher.Timeout = time.Duration(cfg.TimeoutHours) * time.Hour
	}
	if gt == nil {
		gt = web.NewGoroutineTracker()
	}
	gt.Go(ctx, wg, "approvals", func(ctx context.Context) error { return approvalRun(approvalRunContext(ctx), watcher) })
}

func run(args []string, serve func(*http.Server) error) error {
	fs := flag.NewFlagSet("gateway", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	addr := ":8080"
	var database *db.DB
	var approvalsClient approvals.ApprovalClient
	var linearCfg config.LinearConfig
	var cfg config.Config
	var temporalClient client.Client
	if *configPath != "" {
		loaded, err := config.LoadConfig(*configPath)
		if err != nil {
			return err
		}
		cfg = loaded
		if cfg.Gateway.HTTPAddr != "" {
			addr = cfg.Gateway.HTTPAddr
		}
		if cfg.Storage.PostgresDSN != "" {
			database, err = newDB(cfg.Storage.PostgresDSN)
			if err != nil {
				return err
			}
			defer database.Close()
		}
		if cfg.Orchestrator.TemporalAddr != "" {
			tc, err := newTemporalClient(cfg.Orchestrator)
			if err != nil {
				slog.Warn("temporal client connection failed, workflow orchestration disabled", "error", err)
			} else if tc != nil {
				temporalClient = tc
				defer temporalClient.Close()
			}
		}
		linearCfg = cfg.Connectors.Linear
		if linearCfg.Token != "" && linearCfg.TeamID != "" {
			linear := newLinearClient()
			linear.Token = linearCfg.Token
			linear.TeamID = linearCfg.TeamID
			if linearCfg.BaseURL != "" {
				linear.BaseURL = linearCfg.BaseURL
			}
			approvalsClient = linear
		}
	}

	evaluator := &policy.Evaluator{}
	if cfg.Policy.OPAURL != "" {
		evaluator.Checker = newPolicyService(cfg.Policy)
	}
	if cfg.Gateway.OIDCIssuer != "" || cfg.Gateway.OIDCClientID != "" || cfg.Gateway.OIDCJWKSURL != "" || cfg.Gateway.DevMode {
		web.SetAuthConfig(web.AuthConfig{
			Issuer:   cfg.Gateway.OIDCIssuer,
			Audience: cfg.Gateway.OIDCClientID,
			JWKSURL:  cfg.Gateway.OIDCJWKSURL,
			DevMode:  cfg.Gateway.DevMode,
		})
	}
	if cfg.Gateway.JWKSCacheTTLSecs > 0 {
		web.SetJWKSCacheTTL(time.Duration(cfg.Gateway.JWKSCacheTTLSecs) * time.Second)
	}
	srv := newServer(database, evaluator)
	srv.Goroutines = web.NewGoroutineTracker()
	srv.TemporalHealth = func(ctx context.Context) error {
		if temporalClient == nil {
			return nil
		}
		_, err := temporalClient.CheckHealth(ctx, nil)
		return err
	}

	var wg sync.WaitGroup

	srv.AutoApproveLow = cfg.Approvals.AutoApproveLow
	srv.Approvals = approvalsClient
	srv.EnableEventLoop = cfg.Gateway.EnableEventLoop
	srv.EventLoopSources = cfg.Gateway.EventLoopSources
	srv.SessionRequired = cfg.Gateway.SessionRequired
	srv.FailOpenReads = cfg.Policy.FailOpenReads
	srv.WorkspaceDir = cfg.Storage.WorkspaceDir
	if database != nil && cfg.Gateway.EnableEventLoop {
		gate := &web.EventGate{Store: database}
		if cfg.Gateway.EventGateMinCount > 0 {
			gate.MinCount = cfg.Gateway.EventGateMinCount
		}
		if cfg.Gateway.EventGateWindowSecs > 0 {
			gate.Window = time.Duration(cfg.Gateway.EventGateWindowSecs) * time.Second
		} else if cfg.Gateway.AlertDedupWindowSecs > 0 {
			gate.Window = time.Duration(cfg.Gateway.AlertDedupWindowSecs) * time.Second
		}
		if cfg.Gateway.EventGateBackoffSecs > 0 {
			gate.Backoff = time.Duration(cfg.Gateway.EventGateBackoffSecs) * time.Second
		}
		if len(cfg.Gateway.EventGateSeverities) > 0 {
			gate.AllowSeverities = cfg.Gateway.EventGateSeverities
		}
		srv.EventGate = gate
	}
	if cfg.ToolRouter.BaseURL != "" {
		srv.Diagnostics = &web.ToolDiagnostics{
			Router:     newToolRouterClient(cfg.ToolRouter),
			Store:      newObjectStore(cfg.Storage.ObjectStore),
			PresignTTL: 15 * time.Minute,
		}
	}
	if cfg.Gateway.EnableEventLoop && cfg.ToolRouter.BaseURL != "" && cfg.Connectors.Alertmanager.Addr != "" {
		poller := &web.AlertPoller{
			Router:       newToolRouterClient(cfg.ToolRouter),
			Store:        database,
			PollInterval: 30 * time.Second,
			DedupWindow:  2 * time.Minute,
		}
		if cfg.Gateway.AlertPollIntervalSecs > 0 {
			poller.PollInterval = time.Duration(cfg.Gateway.AlertPollIntervalSecs) * time.Second
		}
		if cfg.Gateway.AlertDedupWindowSecs > 0 {
			poller.DedupWindow = time.Duration(cfg.Gateway.AlertDedupWindowSecs) * time.Second
		}
		poller.Handler = func(ctx context.Context, source string, payload map[string]any) error {
			if !srv.ShouldRunEventLoop(source) {
				return nil
			}
			if srv.EventGate != nil {
				allowed, _, err := srv.EventGate.Accept(ctx, source, payload)
				if err != nil {
					return err
				}
				if !allowed {
					return nil
				}
			}
			_, err := srv.RunAlertEventLoop(ctx, source, payload)
			return err
		}
		srv.Goroutines.Go(ctx, &wg, "alert-poller", func(ctx context.Context) error { return poller.Run(ctx) })
	}
	if database != nil && cfg.ToolRouter.BaseURL != "" {
		if store, ok := any(database).(ctxmodel.Store); ok {
			ctxSvc := newContextService(cfg, store, newToolRouterClient(cfg.ToolRouter))
			if ctxSvc != nil {
				ctxSvc.Errs = make(chan error, 16)
				if snapStore, ok := any(database).(ctxmodel.SnapshotWriter); ok {
					ctxSvc.SnapshotWriter = snapStore
				}
				srv.Context = ctxSvc
				srv.Goroutines.Go(ctx, &wg, "context-service", func(ctx context.Context) error {
					if err := ctxSvc.Start(ctx); err != nil {
						return err
					}
					for {
						select {
						case <-ctx.Done():
							return nil
						case err := <-ctxSvc.Errs:
							if err != nil && ctx.Err() == nil {
								return err
							}
						}
					}
				})
			}
		}
	}
	if temporalClient != nil {
		srv.Executor = &workflows.TemporalStarter{
			Client:    temporalClient,
			TaskQueue: cfg.Orchestrator.TaskQueue,
		}
	}
	if cfg.Storage.ObjectStore.Bucket != "" {
		srv.ObjectStore = newObjectStore(cfg.Storage.ObjectStore)
	}
	if cfg.LLM.Provider != "" {
		srv.Planner = newLLMRouter(cfg.LLM)
	}
	if approvalsClient != nil && database != nil {
		startApprovalWatcher(ctx, &wg, srv.Goroutines, approvalsClient, database, linearCfg)
	}
	if database != nil {
		seedWorkflowCatalog(context.Background(), database)
	}
	if cfg.Scheduler.Enabled && database != nil {
		scheduler := web.NewScheduler(database)
		if cfg.Scheduler.PollIntervalSecs > 0 {
			scheduler.PollInterval = time.Duration(cfg.Scheduler.PollIntervalSecs) * time.Second
		}
		startScheduler(ctx, &wg, srv.Goroutines, scheduler)
	}

	mainSrv := &http.Server{Addr: addr, Handler: metrics.Middleware(srv.Mux)}
	errCh := make(chan error, 3)
	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- serve(mainSrv)
	}()

	var canvasSrv *http.Server
	if cfg.Gateway.CanvasAddr != "" && cfg.Gateway.CanvasAddr != addr {
		canvasSrv = &http.Server{Addr: cfg.Gateway.CanvasAddr, Handler: srv.UIHandler()}
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- serve(canvasSrv)
		}()
	}

	var wsSrv *http.Server
	if cfg.Gateway.WSAddr != "" && cfg.Gateway.WSAddr != addr && cfg.Gateway.WSAddr != cfg.Gateway.CanvasAddr {
		wsSrv = &http.Server{Addr: cfg.Gateway.WSAddr, Handler: srv.Mux}
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- serve(wsSrv)
		}()
	}

	slog.Info("gateway listening", "addr", addr)
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
	_ = mainSrv.Shutdown(shutdownCtx)
	if canvasSrv != nil {
		_ = canvasSrv.Shutdown(shutdownCtx)
	}
	if wsSrv != nil {
		_ = wsSrv.Shutdown(shutdownCtx)
	}
	wg.Wait()
	select {
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	default:
		return nil
	}
}

type workflowCatalogStore interface {
	ListWorkflowCatalog(ctx context.Context, limit, offset int) ([]byte, int, error)
	CreateWorkflowCatalog(ctx context.Context, payload []byte) (string, error)
}

func seedWorkflowCatalog(ctx context.Context, store workflowCatalogStore) {
	if store == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("workflow catalog seed skipped", "error", r)
		}
	}()
	payload, _, err := store.ListWorkflowCatalog(ctx, 200, 0)
	if err != nil {
		slog.Warn("workflow catalog list failed", "error", err)
		return
	}
	var existing []map[string]any
	_ = json.Unmarshal(payload, &existing)
	seen := map[string]bool{}
	for _, item := range existing {
		if name, ok := item["name"].(string); ok && name != "" {
			seen[name] = true
		}
	}
	for _, tmpl := range web.DefaultWorkflowCatalog() {
		if seen[tmpl.Name] {
			continue
		}
		spec := web.WorkflowSpecFromTemplate(tmpl)
		data, err := json.Marshal(map[string]any{
			"name":    tmpl.Name,
			"version": 1,
			"spec":    spec,
		})
		if err != nil {
			slog.Warn("workflow catalog encode failed", "error", err)
			continue
		}
		if _, err := store.CreateWorkflowCatalog(ctx, data); err != nil {
			slog.Warn("workflow catalog create failed", "error", err)
		}
	}
}
