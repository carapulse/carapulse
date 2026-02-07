package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"carapulse/internal/approvals"
	"carapulse/internal/config"
	"carapulse/internal/db"
	"carapulse/internal/llm"
	"carapulse/internal/policy"
	"carapulse/internal/web"
	"go.temporal.io/sdk/client"
)

func TestMainPlaceholder(t *testing.T) {
	err := run([]string{}, func(srv *http.Server) error { return nil })
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

type fakeDriver struct{}

type fakeConn struct{}

func (fakeConn) Prepare(query string) (driver.Stmt, error) { return nil, nil }
func (fakeConn) Close() error                              { return nil }
func (fakeConn) Begin() (driver.Tx, error)                 { return nil, nil }

func (fakeDriver) Open(name string) (driver.Conn, error) { return fakeConn{}, nil }

var registerOnce sync.Once

func registerFakeDriver() {
	registerOnce.Do(func() {
		defer func() { _ = recover() }()
		sql.Register("postgres", fakeDriver{})
	})
}

func TestRunWithConfig(t *testing.T) {
	registerFakeDriver()
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":9090","oidc_issuer":"http://issuer","oidc_client_id":"client","oidc_jwks_url":"http://jwks"},"policy":{"opa_url":"http://opa","policy_package":"p"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	oldTemporal := newTemporalClient
	newTemporalClient = func(cfg config.OrchestratorConfig) (client.Client, error) { return nil, nil }
	defer func() { newTemporalClient = oldTemporal }()
	err := run([]string{"-config", file}, func(srv *http.Server) error { return nil })
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestRunAutoApproveLowConfig(t *testing.T) {
	registerFakeDriver()
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":9092"},"policy":{"opa_url":"http://opa","policy_package":"p"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}},"approvals":{"auto_approve_low":true}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	oldServer := newServer
	oldTemporal := newTemporalClient
	defer func() { newServer = oldServer }()
	newTemporalClient = func(cfg config.OrchestratorConfig) (client.Client, error) { return nil, nil }
	defer func() { newTemporalClient = oldTemporal }()

	var captured *web.Server
	newServer = func(database web.DBWriter, evaluator *policy.Evaluator) *web.Server {
		captured = web.NewServer(database, evaluator)
		return captured
	}

	if err := run([]string{"-config", file}, func(srv *http.Server) error { return nil }); err != nil {
		t.Fatalf("err: %v", err)
	}
	if captured == nil || !captured.AutoApproveLow {
		t.Fatalf("auto approve not set")
	}
}

func TestRunSetsPlanner(t *testing.T) {
	registerFakeDriver()
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":9093"},"policy":{"opa_url":"http://opa","policy_package":"p"},"llm":{"provider":"openai","api_key":"k","model":"gpt","max_output_tokens":100,"redact_patterns":["secret=.*"]},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	oldServer := newServer
	oldLLM := newLLMRouter
	oldTemporal := newTemporalClient
	defer func() {
		newServer = oldServer
		newLLMRouter = oldLLM
	}()
	newTemporalClient = func(cfg config.OrchestratorConfig) (client.Client, error) { return nil, nil }
	defer func() { newTemporalClient = oldTemporal }()

	var captured *web.Server
	newServer = func(database web.DBWriter, evaluator *policy.Evaluator) *web.Server {
		captured = web.NewServer(database, evaluator)
		return captured
	}

	var llmCalled bool
	newLLMRouter = func(cfg config.LLMConfig) *llm.Router {
		llmCalled = true
		return &llm.Router{Provider: cfg.Provider}
	}

	if err := run([]string{"-config", file}, func(srv *http.Server) error { return nil }); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !llmCalled {
		t.Fatalf("expected llm router")
	}
	if _, ok := captured.Planner.(*llm.Router); !ok {
		t.Fatalf("planner type: %#v", captured.Planner)
	}
}

func TestNewLLMRouterTimeout(t *testing.T) {
	cfg := config.LLMConfig{
		Provider:        "openai",
		Model:           "gpt",
		APIBase:         "http://example",
		TimeoutMS:       1234,
		MaxOutputTokens: 99,
		RedactPatterns:  []string{"secret=.*"},
		AuthProfile:     "p1",
		AuthPath:        "/tmp/auth.json",
	}
	router := newLLMRouter(cfg)
	if router.Provider != "openai" || router.Model != "gpt" || router.APIBase != "http://example" {
		t.Fatalf("router: %#v", router)
	}
	if router.HTTPClient == nil {
		t.Fatalf("expected http client")
	}
	if router.MaxTokens != 99 || router.AuthProfile != "p1" || router.AuthPath != "/tmp/auth.json" {
		t.Fatalf("router fields: %#v", router)
	}
	if len(router.RedactPatterns) != 1 {
		t.Fatalf("patterns: %#v", router.RedactPatterns)
	}
}

func TestNewPolicyServiceDefaultPackage(t *testing.T) {
	service := newPolicyService(config.PolicyConfig{OPAURL: "http://opa"})
	if service.PolicyPackage != defaultPolicyPackage {
		t.Fatalf("package: %s", service.PolicyPackage)
	}
}

func TestRunPolicyServiceWiring(t *testing.T) {
	registerFakeDriver()
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":9090"},"policy":{"opa_url":"http://opa"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	oldPolicy := newPolicyService
	oldServer := newServer
	oldTemporal := newTemporalClient
	defer func() {
		newPolicyService = oldPolicy
		newServer = oldServer
	}()
	newTemporalClient = func(cfg config.OrchestratorConfig) (client.Client, error) { return nil, nil }
	defer func() { newTemporalClient = oldTemporal }()

	var called bool
	newPolicyService = func(cfg config.PolicyConfig) *policy.PolicyService {
		called = true
		return &policy.PolicyService{OPAURL: cfg.OPAURL, PolicyPackage: "pkg"}
	}
	newServer = func(database web.DBWriter, evaluator *policy.Evaluator) *web.Server {
		if evaluator == nil || evaluator.Checker == nil {
			t.Fatalf("missing checker")
		}
		return web.NewServer(database, evaluator)
	}

	if err := run([]string{"-config", file}, func(srv *http.Server) error { return nil }); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !called {
		t.Fatalf("expected policy service")
	}
}

func TestRunWithLinearConfig(t *testing.T) {
	registerFakeDriver()
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":9091"},"policy":{"opa_url":"http://opa","policy_package":"p"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}},"connectors":{"linear":{"token":"tok","team_id":"team","base_url":"http://linear","poll_interval_ms":1500,"timeout_hours":48}}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	oldServer := newServer
	oldStart := startApprovalWatcher
	oldClient := newLinearClient
	oldTemporal := newTemporalClient
	defer func() {
		newServer = oldServer
		startApprovalWatcher = oldStart
		newLinearClient = oldClient
	}()
	newTemporalClient = func(cfg config.OrchestratorConfig) (client.Client, error) { return nil, nil }
	defer func() { newTemporalClient = oldTemporal }()

	var captured *web.Server
	newServer = func(database web.DBWriter, evaluator *policy.Evaluator) *web.Server {
		captured = web.NewServer(database, evaluator)
		return captured
	}

	var watched struct {
		called  bool
		poll    time.Duration
		timeout time.Duration
	}
	startApprovalWatcher = func(ctx context.Context, wg *sync.WaitGroup, gt *web.GoroutineTracker, client approvals.ApprovalClient, store approvals.ApprovalStore, cfg config.LinearConfig) {
		watched.called = true
		watched.poll = time.Duration(cfg.PollIntervalMS) * time.Millisecond
		watched.timeout = time.Duration(cfg.TimeoutHours) * time.Hour
	}
	newLinearClient = func() *approvals.LinearClient {
		return &approvals.LinearClient{}
	}

	err := run([]string{"-config", file}, func(srv *http.Server) error { return nil })
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if captured == nil || captured.Approvals == nil {
		t.Fatalf("expected approvals client")
	}
	client, ok := captured.Approvals.(*approvals.LinearClient)
	if !ok || client.Token != "tok" || client.TeamID != "team" || client.BaseURL != "http://linear" {
		t.Fatalf("client: %#v", captured.Approvals)
	}
	if !watched.called || watched.poll != 1500*time.Millisecond || watched.timeout != 48*time.Hour {
		t.Fatalf("watcher: %+v", watched)
	}
}

type noopApprovalStore struct{}

func (noopApprovalStore) UpdateApprovalStatusByPlan(ctx context.Context, planID, status string) error {
	return nil
}

func TestStartApprovalWatcher(t *testing.T) {
	oldRun := approvalRun
	defer func() { approvalRun = oldRun }()

	done := make(chan struct{})
	approvalRun = func(ctx context.Context, w *approvals.Watcher) error {
		if w.PollInterval != 2*time.Second {
			t.Fatalf("poll: %v", w.PollInterval)
		}
		if w.Timeout != 3*time.Hour {
			t.Fatalf("timeout: %v", w.Timeout)
		}
		close(done)
		return errors.New("boom")
	}

	var wg sync.WaitGroup
	startApprovalWatcher(context.Background(), &wg, web.NewGoroutineTracker(), &approvals.LinearClient{}, noopApprovalStore{}, config.LinearConfig{PollIntervalMS: 2000, TimeoutHours: 3})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}
}

func TestApprovalRunDefault(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	watcher := approvals.NewWatcher(&approvals.LinearClient{}, noopApprovalStore{})
	if err := approvalRun(ctx, watcher); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunBadConfig(t *testing.T) {
	err := run([]string{"-config", "/no/such/file.json"}, func(srv *http.Server) error { return nil })
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestMainUsesListen(t *testing.T) {
	oldServe := serveHTTP
	serveHTTP = func(srv *http.Server) error { return nil }
	defer func() { serveHTTP = oldServe }()
	oldArgs := os.Args
	os.Args = []string{"gateway"}
	defer func() { os.Args = oldArgs }()
	main()
}

func TestMainFatalOnError(t *testing.T) {
	oldFatal := fatalf
	called := false
	fatalf = func(format string, args ...any) { called = true }
	defer func() { fatalf = oldFatal }()

	oldArgs := os.Args
	os.Args = []string{"gateway", "-badflag"}
	defer func() { os.Args = oldArgs }()

	main()
	if !called {
		t.Fatalf("expected fatal")
	}
}

func TestRunNewDBError(t *testing.T) {
	oldNewDB := newDB
	newDB = func(dsn string) (*db.DB, error) { return nil, errors.New("open") }
	defer func() { newDB = oldNewDB }()

	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":9090"},"policy":{"opa_url":"http://opa","policy_package":"p"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := run([]string{"-config", file}, func(srv *http.Server) error { return nil })
	if err == nil {
		t.Fatalf("expected error")
	}
}
