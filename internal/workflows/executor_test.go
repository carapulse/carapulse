package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"carapulse/internal/db"
	"carapulse/internal/tools"
)

type fakeExecutionStore struct {
	executions    []db.ExecutionRef
	stepsJSON     []byte
	updatedStatus []string
	completed     []string
	toolCalls     []map[string]any
	updatedTool   []map[string]any
	evidence      []map[string]any
	updateErr     error
	completeErr   error
	insertToolErr error
	updateToolErr error
	insertEvidErr error
	listStepsErr  error
	listErr       error
	planJSON      []byte
	planErr       error
}

func (f *fakeExecutionStore) ListExecutionsByStatus(ctx context.Context, status string, limit int) ([]db.ExecutionRef, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.executions, nil
}
func (f *fakeExecutionStore) GetPlan(ctx context.Context, planID string) ([]byte, error) {
	if f.planErr != nil {
		return nil, f.planErr
	}
	return f.planJSON, nil
}
func (f *fakeExecutionStore) ListPlanSteps(ctx context.Context, planID string) ([]byte, error) {
	if f.listStepsErr != nil {
		return nil, f.listStepsErr
	}
	return f.stepsJSON, nil
}
func (f *fakeExecutionStore) UpdateExecutionStatus(ctx context.Context, executionID, status string) error {
	f.updatedStatus = append(f.updatedStatus, status)
	return f.updateErr
}
func (f *fakeExecutionStore) CompleteExecution(ctx context.Context, executionID, status string) error {
	f.completed = append(f.completed, status)
	return f.completeErr
}
func (f *fakeExecutionStore) InsertToolCall(ctx context.Context, executionID string, payload []byte) (string, error) {
	if f.insertToolErr != nil {
		return "", f.insertToolErr
	}
	var data map[string]any
	_ = json.Unmarshal(payload, &data)
	f.toolCalls = append(f.toolCalls, data)
	return "tool_1", nil
}
func (f *fakeExecutionStore) UpdateToolCall(ctx context.Context, toolCallID, status, inputRef, outputRef string) error {
	if f.updateToolErr != nil {
		return f.updateToolErr
	}
	f.updatedTool = append(f.updatedTool, map[string]any{"status": status, "input": inputRef, "output": outputRef})
	return nil
}
func (f *fakeExecutionStore) InsertEvidence(ctx context.Context, executionID string, payload []byte) (string, error) {
	if f.insertEvidErr != nil {
		return "", f.insertEvidErr
	}
	var data map[string]any
	_ = json.Unmarshal(payload, &data)
	f.evidence = append(f.evidence, data)
	return "evid_1", nil
}

type fakeBlobStore struct {
	puts []string
}

func (f *fakeBlobStore) Put(ctx context.Context, key string, data []byte) (string, error) {
	f.puts = append(f.puts, key)
	return "s3://bucket/" + key, nil
}

func (f *fakeBlobStore) Presign(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "https://signed/" + key, nil
}

func withTempCLI(t *testing.T, name string) func() {
	tmp := t.TempDir()
	writeCLI(t, tmp, name)
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	if _, err := exec.LookPath(name); err != nil {
		t.Fatalf("lookpath %s: %v", name, err)
	}
	return func() { _ = os.Setenv("PATH", oldPath) }
}

func TestExecutorRunOnceSuccess(t *testing.T) {
	steps := []map[string]any{{"action": "scale", "tool": "kubectl", "input": map[string]any{"resource": "deploy/app", "replicas": 1}}}
	stepsJSON, _ := json.Marshal(steps)
	store := &fakeExecutionStore{executions: []db.ExecutionRef{{ExecutionID: "exec_1", PlanID: "plan_1"}}, stepsJSON: stepsJSON}
	blob := &fakeBlobStore{}

	tmp := t.TempDir()
	writeCLI(t, tmp, "kubectl")
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() { _ = os.Setenv("PATH", oldPath) }()

	rt := NewRuntime(tools.NewRouter(), &tools.Sandbox{Enforce: false}, tools.HTTPClients{})
	executor := &Executor{Store: store, Runtime: rt, Objects: blob}
	count, err := executor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if count != 1 {
		t.Fatalf("count: %d", count)
	}
	if len(store.completed) == 0 || store.completed[0] != "succeeded" {
		t.Fatalf("completed: %#v", store.completed)
	}
	if len(store.updatedTool) == 0 {
		t.Fatalf("tool calls missing")
	}
	if len(store.evidence) != 0 {
		// kubectl does not produce evidence
	} else {
		// ok
	}
	if len(blob.puts) == 0 {
		t.Fatalf("expected object store puts")
	}
}

func TestExecutorRunOnceFailureRollback(t *testing.T) {
	steps := []map[string]any{{"action": "scale", "tool": "kubectl", "input": map[string]any{"resource": "deploy/app", "replicas": 1}, "rollback": map[string]any{"tool": "kubectl", "action": "rollout-status", "input": map[string]any{"resource": "deploy/app"}}}}
	stepsJSON, _ := json.Marshal(steps)
	store := &fakeExecutionStore{executions: []db.ExecutionRef{{ExecutionID: "exec_1", PlanID: "plan_1"}}, stepsJSON: stepsJSON}

	tmp := t.TempDir()
	writeCLI(t, tmp, "kubectl")
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() { _ = os.Setenv("PATH", oldPath) }()

	calls := 0
	sandbox := &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("boom")
		}
		return []byte("ok"), nil
	}}
	router := tools.NewRouter()
	rt := NewRuntime(router, sandbox, tools.HTTPClients{})
	executor := &Executor{Store: store, Runtime: rt}
	_, err := executor.RunOnce(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(store.completed) == 0 || store.completed[0] != "rolled_back" {
		t.Fatalf("completed: %#v", store.completed)
	}
}

func TestExecutorRunOnceNoExecutions(t *testing.T) {
	executor := &Executor{Store: &fakeExecutionStore{}, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	count, err := executor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if count != 0 {
		t.Fatalf("count: %d", count)
	}
}

func TestExecutorRunOnceMissingRuntime(t *testing.T) {
	executor := &Executor{Store: &fakeExecutionStore{}}
	if _, err := executor.RunOnce(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorRunOnceListError(t *testing.T) {
	executor := &Executor{
		Store:   &fakeExecutionStore{listErr: errors.New("boom")},
		Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{}),
	}
	if _, err := executor.RunOnce(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorRunContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	executor := &Executor{Store: &fakeExecutionStore{}, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	if err := executor.Run(ctx); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorRunNilContext(t *testing.T) {
	executor := &Executor{Store: &fakeExecutionStore{}, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	if err := executor.Run(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorRunMissingDeps(t *testing.T) {
	if err := (&Executor{Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}).Run(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
	if err := (&Executor{Store: &fakeExecutionStore{}}).Run(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorRunSelectCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	executor := &Executor{Store: &fakeExecutionStore{}, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{}), PollInterval: time.Millisecond}
	go func() {
		time.Sleep(2 * time.Millisecond)
		cancel()
	}()
	if err := executor.Run(ctx); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorLoadStepsError(t *testing.T) {
	store := &fakeExecutionStore{
		executions:   []db.ExecutionRef{{ExecutionID: "exec_1", PlanID: "plan_1"}},
		listStepsErr: errors.New("boom"),
	}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	if _, err := executor.RunOnce(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
	if len(store.completed) == 0 || store.completed[0] != "failed" {
		t.Fatalf("completed: %#v", store.completed)
	}
}

func TestExecutorLoadStepsEmpty(t *testing.T) {
	store := &fakeExecutionStore{stepsJSON: []byte{}}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	steps, err := executor.loadSteps(context.Background(), "plan_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if steps != nil {
		t.Fatalf("steps: %#v", steps)
	}
}

func TestExecutorLoadStepsBadJSON(t *testing.T) {
	store := &fakeExecutionStore{stepsJSON: []byte(`{`)}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	if _, err := executor.loadSteps(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorLoadContextError(t *testing.T) {
	store := &fakeExecutionStore{planErr: errors.New("boom")}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	if _, err := executor.loadContext(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorLoadContextBadJSON(t *testing.T) {
	store := &fakeExecutionStore{planJSON: []byte("{")}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	if _, err := executor.loadContext(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorLoadContextOK(t *testing.T) {
	store := &fakeExecutionStore{planJSON: []byte(`{"context":{"tenant_id":"t","environment":"prod"}}`)}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	ctxRef, err := executor.loadContext(context.Background(), "plan_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ctxRef.TenantID != "t" || ctxRef.Environment != "prod" {
		t.Fatalf("ctx: %#v", ctxRef)
	}
}

func TestExecutorExecutePlanUpdateError(t *testing.T) {
	store := &fakeExecutionStore{
		executions: []db.ExecutionRef{{ExecutionID: "exec_1", PlanID: "plan_1"}},
		updateErr:  errors.New("boom"),
		stepsJSON:  []byte(`[]`),
	}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	if _, err := executor.RunOnce(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

type failingBlobStore struct{}

func (f failingBlobStore) Put(ctx context.Context, key string, data []byte) (string, error) {
	return "", errors.New("fail")
}

func (f failingBlobStore) Presign(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "", errors.New("fail")
}

type outputFailBlobStore struct {
	inputCalls  int
	outputCalls int
	lastKey     string
}

func (o *outputFailBlobStore) Put(ctx context.Context, key string, data []byte) (string, error) {
	o.lastKey = key
	if strings.HasPrefix(key, "tool-output/") {
		o.outputCalls++
		return "", errors.New("fail")
	}
	o.inputCalls++
	return "s3://bucket/" + key, nil
}

func (o *outputFailBlobStore) Presign(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "https://signed/" + key, nil
}

func TestExecutorExecuteStepStoreError(t *testing.T) {
	store := &fakeExecutionStore{}
	executor := &Executor{
		Store:   store,
		Runtime: NewRuntime(tools.NewRouter(), &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) { return []byte("ok"), nil }}, tools.HTTPClients{}),
		Objects: failingBlobStore{},
	}
	step := PlanStep{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "deploy/app", "replicas": 1}}
	if err := executor.executeStep(context.Background(), "exec_1", step, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorExecuteStepStoreOutputError(t *testing.T) {
	resetPath := withTempCLI(t, "kubectl")
	defer resetPath()

	store := &fakeExecutionStore{}
	blob := &outputFailBlobStore{}
	executor := &Executor{
		Store:   store,
		Runtime: NewRuntime(tools.NewRouter(), &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) { return []byte("ok"), nil }}, tools.HTTPClients{}),
		Objects: blob,
	}
	step := PlanStep{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "deploy/app", "replicas": 1}}
	if err := executor.executeStep(context.Background(), "exec_1", step, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
	if blob.outputCalls == 0 {
		t.Fatalf("expected output call")
	}
}

func TestExecutorExecuteStepInsertToolError(t *testing.T) {
	store := &fakeExecutionStore{insertToolErr: errors.New("fail")}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	step := PlanStep{Tool: "kubectl", Action: "scale"}
	if err := executor.executeStep(context.Background(), "exec_1", step, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorExecuteStepEvidenceError(t *testing.T) {
	resetPath := withTempCLI(t, "kubectl")
	defer resetPath()

	store := &fakeExecutionStore{insertEvidErr: errors.New("boom")}
	executor := &Executor{
		Store:   store,
		Runtime: NewRuntime(tools.NewRouter(), &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) { return []byte("ok"), nil }}, tools.HTTPClients{}),
	}
	step := PlanStep{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "deploy/app", "replicas": 1}}
	if err := executor.executeStep(context.Background(), "exec_1", step, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorExecuteStepUpdateToolError(t *testing.T) {
	store := &fakeExecutionStore{updateToolErr: errors.New("boom")}
	executor := &Executor{
		Store:   store,
		Runtime: NewRuntime(tools.NewRouter(), &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) { return []byte("ok"), nil }}, tools.HTTPClients{}),
		Objects: &fakeBlobStore{},
	}
	step := PlanStep{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "deploy/app", "replicas": 1}}
	if err := executor.executeStep(context.Background(), "exec_1", step, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorExecuteStepRunToolError(t *testing.T) {
	store := &fakeExecutionStore{}
	executor := &Executor{
		Store:   store,
		Runtime: NewRuntime(tools.NewRouter(), &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) { return nil, errors.New("boom") }}, tools.HTTPClients{}),
	}
	step := PlanStep{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "deploy/app", "replicas": 1}}
	if err := executor.executeStep(context.Background(), "exec_1", step, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorExecutePlanMissingIDs(t *testing.T) {
	executor := &Executor{Store: &fakeExecutionStore{}, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	if err := executor.executePlan(context.Background(), db.ExecutionRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorTryRollbackMissing(t *testing.T) {
	executor := &Executor{Store: &fakeExecutionStore{}, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	if err := executor.tryRollback(context.Background(), "exec_1", PlanStep{}, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorTryRollbackInvalid(t *testing.T) {
	executor := &Executor{Store: &fakeExecutionStore{}, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	step := PlanStep{Rollback: map[string]any{"tool": "", "action": ""}}
	if err := executor.tryRollback(context.Background(), "exec_1", step, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorTryRollbackInsertToolError(t *testing.T) {
	store := &fakeExecutionStore{insertToolErr: errors.New("boom")}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	step := PlanStep{Rollback: map[string]any{"tool": "kubectl", "action": "scale", "input": map[string]any{"resource": "deploy/app", "replicas": 1}}}
	if err := executor.tryRollback(context.Background(), "exec_1", step, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorTryRollbackStoreInputError(t *testing.T) {
	store := &fakeExecutionStore{}
	executor := &Executor{
		Store:   store,
		Runtime: NewRuntime(tools.NewRouter(), &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) { return []byte("ok"), nil }}, tools.HTTPClients{}),
		Objects: failingBlobStore{},
	}
	step := PlanStep{Rollback: map[string]any{"tool": "kubectl", "action": "scale", "input": map[string]any{"resource": "deploy/app"}}}
	if err := executor.tryRollback(context.Background(), "exec_1", step, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorTryRollbackStoreOutputError(t *testing.T) {
	resetPath := withTempCLI(t, "kubectl")
	defer resetPath()

	store := &fakeExecutionStore{}
	blob := &outputFailBlobStore{}
	executor := &Executor{
		Store:   store,
		Runtime: NewRuntime(tools.NewRouter(), &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) { return []byte("ok"), nil }}, tools.HTTPClients{}),
		Objects: blob,
	}
	step := PlanStep{Rollback: map[string]any{"tool": "kubectl", "action": "scale", "input": map[string]any{"resource": "deploy/app", "replicas": 1}}}
	if err := executor.tryRollback(context.Background(), "exec_1", step, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
	if blob.outputCalls == 0 {
		t.Fatalf("expected output call")
	}
}

func TestExecutorTryRollbackRunToolError(t *testing.T) {
	store := &fakeExecutionStore{}
	executor := &Executor{
		Store:   store,
		Runtime: NewRuntime(tools.NewRouter(), &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) { return nil, errors.New("boom") }}, tools.HTTPClients{}),
	}
	step := PlanStep{Rollback: map[string]any{"tool": "kubectl", "action": "scale", "input": map[string]any{"resource": "deploy/app"}}}
	if err := executor.tryRollback(context.Background(), "exec_1", step, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorTryRollbackUpdateToolError(t *testing.T) {
	store := &fakeExecutionStore{updateToolErr: errors.New("boom")}
	executor := &Executor{
		Store:   store,
		Runtime: NewRuntime(tools.NewRouter(), &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) { return []byte("ok"), nil }}, tools.HTTPClients{}),
		Objects: &fakeBlobStore{},
	}
	step := PlanStep{Rollback: map[string]any{"tool": "kubectl", "action": "scale", "input": map[string]any{"resource": "deploy/app"}}}
	if err := executor.tryRollback(context.Background(), "exec_1", step, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorRecordEvidenceError(t *testing.T) {
	store := &fakeExecutionStore{insertEvidErr: errors.New("boom")}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{}), Objects: failingBlobStore{}}
	step := PlanStep{Tool: "prometheus", Input: map[string]any{"query": "up"}}
	if err := executor.recordEvidence(context.Background(), "exec_1", step, "s3://bucket/key", []byte(`{"Events":[{"EventId":"e1"}]}`)); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorRecordEvidenceSkip(t *testing.T) {
	executor := &Executor{Store: &fakeExecutionStore{}, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	step := PlanStep{Tool: "unknown"}
	if err := executor.recordEvidence(context.Background(), "exec_1", step, "", nil); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestExecutorRecordEvidencePresign(t *testing.T) {
	store := &fakeExecutionStore{}
	blob := &fakeBlobStore{}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{}), Objects: blob}
	step := PlanStep{Tool: "prometheus", Input: map[string]any{"query": "up"}}
	if err := executor.recordEvidence(context.Background(), "exec_1", step, "s3://bucket/key", []byte(`{"id":123}`)); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(store.evidence) == 0 || store.evidence[0]["link"] == "" {
		t.Fatalf("evidence: %#v", store.evidence)
	}
}

func TestExecutorRecordEvidenceNoObjectStore(t *testing.T) {
	store := &fakeExecutionStore{}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	step := PlanStep{Tool: "prometheus", Input: map[string]any{"query": "up"}}
	if err := executor.recordEvidence(context.Background(), "exec_1", step, "s3://bucket/key", []byte(`{"traceId":"abc"}`)); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(store.evidence) == 0 {
		t.Fatalf("expected evidence")
	}
}

func TestExecutorRecordEvidenceNoOutputRef(t *testing.T) {
	store := &fakeExecutionStore{}
	executor := &Executor{Store: store, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{}), Objects: &fakeBlobStore{}}
	step := PlanStep{Tool: "prometheus", Input: map[string]any{"query": "up"}}
	if err := executor.recordEvidence(context.Background(), "exec_1", step, "", nil); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(store.evidence) == 0 {
		t.Fatalf("expected evidence")
	}
	if store.evidence[0]["link"] != "" || store.evidence[0]["result_ref"] != "" {
		t.Fatalf("evidence: %#v", store.evidence[0])
	}
}

func TestExecutorStoreInputNoObjectStore(t *testing.T) {
	executor := &Executor{}
	ref, err := executor.storeInput(context.Background(), "exec", "tool", map[string]any{"a": "b"})
	if err != nil || ref != "" {
		t.Fatalf("ref: %s err: %v", ref, err)
	}
}

func TestExecutorStoreInputMarshalError(t *testing.T) {
	executor := &Executor{Objects: &fakeBlobStore{}}
	_, err := executor.storeInput(context.Background(), "exec", "tool", map[string]any{"bad": make(chan int)})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorStoreOutputNoObjectStore(t *testing.T) {
	executor := &Executor{}
	ref, err := executor.storeOutput(context.Background(), "exec", "tool", []byte("ok"))
	if err != nil || ref != "" {
		t.Fatalf("ref: %s err: %v", ref, err)
	}
}

func TestExecutorInsertToolCallError(t *testing.T) {
	store := &fakeExecutionStore{insertToolErr: errors.New("boom")}
	executor := &Executor{Store: store}
	if _, err := executor.insertToolCall(context.Background(), "exec", "kubectl", "running", "", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorInsertToolCallMarshalError(t *testing.T) {
	old := marshalToolCall
	marshalToolCall = func(v any) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { marshalToolCall = old })

	executor := &Executor{Store: &fakeExecutionStore{}}
	if _, err := executor.insertToolCall(context.Background(), "exec", "kubectl", "running", "", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorRunToolMissingRuntime(t *testing.T) {
	executor := &Executor{}
	if _, err := executor.runTool(context.Background(), "exec", "tool", "kubectl", "scale", nil, tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorRecordEvidenceMarshalError(t *testing.T) {
	old := marshalEvidence
	marshalEvidence = func(v any) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { marshalEvidence = old })

	executor := &Executor{Store: &fakeExecutionStore{}, Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})}
	step := PlanStep{Tool: "prometheus", Input: map[string]any{"query": "up"}}
	if err := executor.recordEvidence(context.Background(), "exec_1", step, "", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorRunOnceMissingDeps(t *testing.T) {
	if _, err := (&Executor{}).RunOnce(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorRunRunOnceError(t *testing.T) {
	executor := &Executor{
		Store:   &fakeExecutionStore{listErr: errors.New("boom")},
		Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{}),
	}
	if err := executor.Run(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecutorStepTimeoutExpired(t *testing.T) {
	resetPath := withTempCLI(t, "kubectl")
	defer resetPath()

	store := &fakeExecutionStore{
		executions: []db.ExecutionRef{{ExecutionID: "exec_1", PlanID: "plan_1"}},
		stepsJSON: func() []byte {
			steps := []map[string]any{{"action": "scale", "tool": "kubectl", "input": map[string]any{"resource": "deploy/app", "replicas": 1}}}
			b, _ := json.Marshal(steps)
			return b
		}(),
	}

	// RunFunc blocks until context is canceled, simulating a long-running tool.
	sandbox := &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	rt := NewRuntime(tools.NewRouter(), sandbox, tools.HTTPClients{})
	executor := &Executor{
		Store:       store,
		Runtime:     rt,
		StepTimeout: 50 * time.Millisecond,
	}
	_, err := executor.RunOnce(context.Background())
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected deadline exceeded, got: %v", err)
	}
	if len(store.completed) == 0 || store.completed[0] != "failed" {
		t.Fatalf("completed: %#v", store.completed)
	}
}

func TestExecutorStepTimeoutDefault(t *testing.T) {
	executor := &Executor{
		Store:   &fakeExecutionStore{},
		Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{}),
	}
	// When Run is called, StepTimeout should be set to default 5 minutes.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so Run returns
	_ = executor.Run(ctx)
	if executor.StepTimeout != 5*time.Minute {
		t.Fatalf("StepTimeout: got %v, want 5m", executor.StepTimeout)
	}
}

func TestExecutorStepTimeoutCustom(t *testing.T) {
	executor := &Executor{
		Store:       &fakeExecutionStore{},
		Runtime:     NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{}),
		StepTimeout: 10 * time.Minute,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = executor.Run(ctx)
	if executor.StepTimeout != 10*time.Minute {
		t.Fatalf("StepTimeout: got %v, want 10m", executor.StepTimeout)
	}
}

func TestEvidenceTypeMapping(t *testing.T) {
	if evidenceType("prometheus") != "promql" {
		t.Fatalf("prometheus")
	}
	if evidenceType("tempo") != "traceql" {
		t.Fatalf("tempo")
	}
	if evidenceType("argocd") != "argocd" {
		t.Fatalf("argocd")
	}
	if evidenceType("grafana") != "grafana" {
		t.Fatalf("grafana")
	}
	if evidenceType("aws") != "cloudtrail" {
		t.Fatalf("aws")
	}
	if evidenceType("kubectl") != "k8s" {
		t.Fatalf("kubectl")
	}
	if evidenceType("unknown") != "" {
		t.Fatalf("unknown")
	}
}
