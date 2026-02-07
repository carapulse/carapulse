package main

import (
	"errors"
	"io"
	"os"
	"testing"

	"carapulse/internal/config"
	"carapulse/internal/db"
	"carapulse/internal/storage"
	"carapulse/internal/workflows"
	"github.com/nexus-rpc/sdk-go/nexus"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type fakeWorker struct {
	workflowCount int
	activityCount int
	ran           bool
}

func (f *fakeWorker) RegisterWorkflow(fn any) {
	f.workflowCount++
}

func (f *fakeWorker) RegisterWorkflowWithOptions(fn any, _ workflow.RegisterOptions) {
	f.workflowCount++
}

func (f *fakeWorker) RegisterDynamicWorkflow(_ any, _ workflow.DynamicRegisterOptions) {}

func (f *fakeWorker) RegisterActivity(fn any) {
	f.activityCount++
}

func (f *fakeWorker) RegisterActivityWithOptions(fn any, _ activity.RegisterOptions) {
	f.activityCount++
}

func (f *fakeWorker) RegisterDynamicActivity(_ any, _ activity.DynamicRegisterOptions) {}
func (f *fakeWorker) RegisterNexusService(_ *nexus.Service)                             {}
func (f *fakeWorker) Start() error                                                      { return nil }
func (f *fakeWorker) Run(<-chan interface{}) error                                     { return nil }
func (f *fakeWorker) Stop()                                                             {}

func TestRunMissingConfig(t *testing.T) {
	if err := run([]string{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunBadFlag(t *testing.T) {
	if err := run([]string{"-badflag"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLoadConfigError(t *testing.T) {
	oldLoad := loadConfig
	loadConfig = func(path string) (config.Config, error) { return config.Config{}, errors.New("boom") }
	defer func() { loadConfig = oldLoad }()

	if err := run([]string{"-config", "cfg.json"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunMissingDSN(t *testing.T) {
	oldLoad := loadConfig
	loadConfig = func(path string) (config.Config, error) {
		return config.Config{}, nil
	}
	defer func() { loadConfig = oldLoad }()
	if err := run([]string{"-config", "cfg.json"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunDBError(t *testing.T) {
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":8080"},"policy":{"opa_url":"http://opa","policy_package":"p"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn"}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	oldDB := newDB
	newDB = func(dsn string) (*db.DB, error) { return nil, errors.New("db fail") }
	defer func() { newDB = oldDB }()
	if err := run([]string{"-config", file}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunOK(t *testing.T) {
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":8080"},"policy":{"opa_url":"http://opa","policy_package":"p"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}},"connectors":{"prometheus":{"addr":"http://prom","token":"ptok"},"thanos":{"addr":"http://thanos","token":"ttok"},"grafana":{"addr":"http://grafana","token":"gtok","org_id":"2"},"tempo":{"addr":"http://tempo","token":"tempotok"},"linear":{"token":"tok","base_url":"http://linear"},"pagerduty":{"addr":"http://pd","token":"pdtok"}}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	oldStart := startWorker
	defer func() { startWorker = oldStart }()
	oldDB := newDB
	newDB = func(dsn string) (*db.DB, error) { return &db.DB{}, nil }
	defer func() { newDB = oldDB }()

	var got *workflows.Runtime
	startWorker = func(rt *workflows.Runtime, store workflows.ExecutionStore, obj *storage.ObjectStore, cfg config.Config) error {
		got = rt
		if cfg.Orchestrator.TemporalAddr != "t" {
			t.Fatalf("temporal: %s", cfg.Orchestrator.TemporalAddr)
		}
		if store == nil || obj == nil {
			t.Fatalf("store: %#v obj: %#v", store, obj)
		}
		return nil
	}

	if err := run([]string{"-config", file}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got == nil || got.Router == nil || got.Sandbox == nil || got.Clients.Linear == nil {
		t.Fatalf("runtime: %#v", got)
	}
	if got.Clients.Linear.BaseURL != "http://linear" {
		t.Fatalf("linear: %s", got.Clients.Linear.BaseURL)
	}
	if got.Clients.Grafana.BaseURL != "http://grafana" {
		t.Fatalf("grafana: %s", got.Clients.Grafana.BaseURL)
	}
	if got.Clients.Tempo.BaseURL != "http://tempo" {
		t.Fatalf("tempo: %s", got.Clients.Tempo.BaseURL)
	}
	if got.Clients.Prometheus.BaseURL != "http://prom" {
		t.Fatalf("prometheus: %s", got.Clients.Prometheus.BaseURL)
	}
	if got.Clients.Thanos.BaseURL != "http://thanos" {
		t.Fatalf("thanos: %s", got.Clients.Thanos.BaseURL)
	}
	if got.Clients.PagerDuty.BaseURL != "http://pd" {
		t.Fatalf("pagerduty: %s", got.Clients.PagerDuty.BaseURL)
	}
	if got.Clients.Grafana.Auth.Extra["X-Grafana-Org-Id"] != "2" {
		t.Fatalf("grafana org: %s", got.Clients.Grafana.Auth.Extra["X-Grafana-Org-Id"])
	}
	if got.Clients.Prometheus.Auth.BearerToken != "ptok" {
		t.Fatalf("prom token: %s", got.Clients.Prometheus.Auth.BearerToken)
	}
}

func TestRunStartWorkerError(t *testing.T) {
	file := t.TempDir() + "/cfg.json"
	data := `{"gateway":{"http_addr":":8080"},"policy":{"opa_url":"http://opa","policy_package":"p"},"orchestrator":{"temporal_addr":"t","namespace":"n","task_queue":"q"},"storage":{"postgres_dsn":"dsn","object_store":{"endpoint":"e","bucket":"b"}}}`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	oldStart := startWorker
	startWorker = func(rt *workflows.Runtime, store workflows.ExecutionStore, obj *storage.ObjectStore, cfg config.Config) error {
		return errors.New("boom")
	}
	defer func() { startWorker = oldStart }()
	oldDB := newDB
	newDB = func(dsn string) (*db.DB, error) { return &db.DB{}, nil }
	defer func() { newDB = oldDB }()
	if err := run([]string{"-config", file}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestStartWorkerDefault(t *testing.T) {
	oldWorker := newWorker
	oldRun := runWorker
	oldSet := setTemporalHealthClient
	defer func() {
		newWorker = oldWorker
		runWorker = oldRun
		setTemporalHealthClient = oldSet
	}()
	fake := &fakeWorker{}
	newWorker = func(cfg config.OrchestratorConfig) (worker.Worker, io.Closer, error) {
		return fake, io.NopCloser(nil), nil
	}
	setTemporalHealthClient = func(c client.Client) {}
	runWorker = func(w worker.Worker) error {
		fake.ran = true
		return nil
	}
	cfg := config.Config{Orchestrator: config.OrchestratorConfig{TemporalAddr: "t", TaskQueue: "q"}}
	if err := startWorker(&workflows.Runtime{}, &db.DB{}, &storage.ObjectStore{}, cfg); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !fake.ran || fake.workflowCount == 0 || fake.activityCount == 0 {
		t.Fatalf("worker not registered")
	}
}

func TestMainFatalOnError(t *testing.T) {
	oldFatal := fatalf
	called := false
	fatalf = func(format string, args ...any) { called = true }
	defer func() { fatalf = oldFatal }()

	oldArgs := os.Args
	os.Args = []string{"orchestrator"}
	defer func() { os.Args = oldArgs }()

	main()
	if !called {
		t.Fatalf("expected fatal")
	}
}
