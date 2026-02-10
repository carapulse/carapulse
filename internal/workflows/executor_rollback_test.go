package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"carapulse/internal/db"
	"carapulse/internal/tools"
)

func TestExecutorRollbackStepsSuccess(t *testing.T) {
	resetPath := withTempCLI(t, "kubectl")
	defer resetPath()

	store := &fakeExecutionStore{}
	blob := &fakeBlobStore{}
	executor := &Executor{
		Store:   store,
		Runtime: NewRuntime(tools.NewRouter(), &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) { return []byte("ok"), nil }}, tools.HTTPClients{}),
		Objects: blob,
	}
	steps := []PlanStep{
		{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "deploy/app", "replicas": 1}, Rollback: map[string]any{"tool": "kubectl", "action": "scale", "input": map[string]any{"resource": "deploy/app", "replicas": 2}}},
		{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "deploy/web", "replicas": 1}, Rollback: map[string]any{"tool": "kubectl", "action": "scale", "input": map[string]any{"resource": "deploy/web", "replicas": 3}}},
	}
	err := executor.rollbackSteps(context.Background(), "exec_1", steps, tools.ContextRef{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestExecutorRollbackStepsPartialFailure(t *testing.T) {
	resetPath := withTempCLI(t, "kubectl")
	defer resetPath()

	calls := 0
	store := &fakeExecutionStore{}
	executor := &Executor{
		Store: store,
		Runtime: NewRuntime(tools.NewRouter(), &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("rollback fail")
			}
			return []byte("ok"), nil
		}}, tools.HTTPClients{}),
		Objects: &fakeBlobStore{},
	}
	steps := []PlanStep{
		{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "deploy/app"}, Rollback: map[string]any{"tool": "kubectl", "action": "scale", "input": map[string]any{"resource": "deploy/app"}}},
		{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "deploy/web"}, Rollback: map[string]any{"tool": "kubectl", "action": "scale", "input": map[string]any{"resource": "deploy/web"}}},
	}
	err := executor.rollbackSteps(context.Background(), "exec_1", steps, tools.ContextRef{})
	if err == nil {
		t.Fatalf("expected error from partial rollback failure")
	}
}

func TestExecutorRollbackStepsEmpty(t *testing.T) {
	executor := &Executor{
		Store:   &fakeExecutionStore{},
		Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{}),
	}
	err := executor.rollbackSteps(context.Background(), "exec_1", nil, tools.ContextRef{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestExecutorVerifyStepFailureTrigersRollback(t *testing.T) {
	resetPath := withTempCLI(t, "kubectl")
	defer resetPath()

	calls := 0
	steps := []map[string]any{
		{"action": "scale", "tool": "kubectl", "stage": "act", "input": map[string]any{"resource": "deploy/app", "replicas": 1}, "rollback": map[string]any{"tool": "kubectl", "action": "scale", "input": map[string]any{"resource": "deploy/app", "replicas": 2}}},
		{"action": "query", "tool": "kubectl", "stage": "verify", "input": map[string]any{"resource": "deploy/app"}},
	}
	stepsJSON, _ := json.Marshal(steps)
	store := &fakeExecutionStore{executions: []db.ExecutionRef{{ExecutionID: "exec_1", PlanID: "plan_1"}}, stepsJSON: stepsJSON}

	executor := &Executor{
		Store: store,
		Runtime: NewRuntime(tools.NewRouter(), &tools.Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) {
			calls++
			if calls == 2 {
				return nil, errors.New("verify failed")
			}
			return []byte("ok"), nil
		}}, tools.HTTPClients{}),
		Objects: &fakeBlobStore{},
	}
	_, err := executor.RunOnce(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(store.completed) == 0 {
		t.Fatalf("expected completion")
	}
	lastStatus := store.completed[len(store.completed)-1]
	if lastStatus != "rolled_back" && lastStatus != "failed" {
		t.Fatalf("unexpected status: %s", lastStatus)
	}
}

func TestExecutorCompleteExecutionError(t *testing.T) {
	steps := []map[string]any{{"action": "scale", "tool": "kubectl", "input": map[string]any{"resource": "deploy/app", "replicas": 1}}}
	stepsJSON, _ := json.Marshal(steps)
	store := &fakeExecutionStore{
		executions:  []db.ExecutionRef{{ExecutionID: "exec_1", PlanID: "plan_1"}},
		stepsJSON:   stepsJSON,
		completeErr: errors.New("complete fail"),
	}
	resetPath := withTempCLI(t, "kubectl")
	defer resetPath()

	executor := &Executor{
		Store:   store,
		Runtime: NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{}),
		Objects: &fakeBlobStore{},
	}
	_, err := executor.RunOnce(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
}
