package workflows

import (
	"context"
	"errors"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestPlanExecutionWorkflowSuccess(t *testing.T) {
	var statuses []string
	var stepCalls int

	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(PlanExecutionWorkflow)
	env.RegisterActivityWithOptions(func(ctx context.Context, executionID, status string) error {
		statuses = append(statuses, status)
		return nil
	}, activity.RegisterOptions{Name: "UpdateExecutionStatus"})
	env.RegisterActivityWithOptions(func(ctx context.Context, executionID, status string) error {
		statuses = append(statuses, status)
		return nil
	}, activity.RegisterOptions{Name: "CompleteExecution"})
	env.RegisterActivityWithOptions(func(ctx context.Context, input StepActivityInput) error {
		stepCalls++
		return nil
	}, activity.RegisterOptions{Name: "ExecuteStep"})
	env.RegisterActivityWithOptions(func(ctx context.Context, input StepActivityInput) error {
		return nil
	}, activity.RegisterOptions{Name: "RollbackStep"})

	input := PlanExecutionInput{
		PlanID:      "plan_1",
		ExecutionID: "exec_1",
		Steps: []PlanStep{
			{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "deploy/app", "replicas": 1}},
		},
	}
	env.ExecuteWorkflow(PlanExecutionWorkflow, input)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow err: %v", err)
	}
	if stepCalls != 1 {
		t.Fatalf("step calls: %d", stepCalls)
	}
	if len(statuses) < 2 || statuses[0] != "running" || statuses[len(statuses)-1] != "succeeded" {
		t.Fatalf("statuses: %#v", statuses)
	}
}

func TestPlanExecutionWorkflowFailure(t *testing.T) {
	var rolledBack bool
	var completed string

	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(PlanExecutionWorkflow)
	env.RegisterActivityWithOptions(func(ctx context.Context, executionID, status string) error {
		return nil
	}, activity.RegisterOptions{Name: "UpdateExecutionStatus"})
	env.RegisterActivityWithOptions(func(ctx context.Context, executionID, status string) error {
		completed = status
		return nil
	}, activity.RegisterOptions{Name: "CompleteExecution"})
	env.RegisterActivityWithOptions(func(ctx context.Context, input StepActivityInput) error {
		return errors.New("boom")
	}, activity.RegisterOptions{Name: "ExecuteStep"})
	env.RegisterActivityWithOptions(func(ctx context.Context, input StepActivityInput) error {
		rolledBack = true
		return nil
	}, activity.RegisterOptions{Name: "RollbackStep"})

	input := PlanExecutionInput{PlanID: "plan_1", ExecutionID: "exec_1", Steps: []PlanStep{{Tool: "kubectl", Action: "scale"}}}
	env.ExecuteWorkflow(PlanExecutionWorkflow, input)
	if err := env.GetWorkflowError(); err == nil {
		t.Fatalf("expected error")
	}
	if !rolledBack || completed != "failed" {
		t.Fatalf("rollback: %v completed: %s", rolledBack, completed)
	}
}
