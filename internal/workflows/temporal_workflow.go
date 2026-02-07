package workflows

import (
	"errors"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type StepActivityInput struct {
	PlanID      string
	ExecutionID string
	Context     ContextRef
	Step        PlanStep
}

// PlanExecutionWorkflow runs plan steps as activities with retries.
func PlanExecutionWorkflow(ctx workflow.Context, input PlanExecutionInput) error {
	if input.ExecutionID == "" {
		return errors.New("execution_id required")
	}
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	if err := workflow.ExecuteActivity(ctx, "UpdateExecutionStatus", input.ExecutionID, "running").Get(ctx, nil); err != nil {
		return err
	}
	actSteps, verifySteps := splitStepsByStage(input.Steps)
	for _, step := range actSteps {
		actInput := StepActivityInput{
			PlanID:      input.PlanID,
			ExecutionID: input.ExecutionID,
			Context:     input.Context,
			Step:        step,
		}
		if err := workflow.ExecuteActivity(ctx, "ExecuteStep", actInput).Get(ctx, nil); err != nil {
			_ = workflow.ExecuteActivity(ctx, "RollbackStep", actInput).Get(ctx, nil)
			_ = workflow.ExecuteActivity(ctx, "CompleteExecution", input.ExecutionID, "failed").Get(ctx, nil)
			return err
		}
	}
	for _, step := range verifySteps {
		verifyInput := StepActivityInput{
			PlanID:      input.PlanID,
			ExecutionID: input.ExecutionID,
			Context:     input.Context,
			Step:        step,
		}
		if err := workflow.ExecuteActivity(ctx, "ExecuteStep", verifyInput).Get(ctx, nil); err != nil {
			for i := len(actSteps) - 1; i >= 0; i-- {
				rollbackInput := StepActivityInput{
					PlanID:      input.PlanID,
					ExecutionID: input.ExecutionID,
					Context:     input.Context,
					Step:        actSteps[i],
				}
				_ = workflow.ExecuteActivity(ctx, "RollbackStep", rollbackInput).Get(ctx, nil)
			}
			_ = workflow.ExecuteActivity(ctx, "CompleteExecution", input.ExecutionID, "failed").Get(ctx, nil)
			return err
		}
	}
	if err := workflow.ExecuteActivity(ctx, "CompleteExecution", input.ExecutionID, "succeeded").Get(ctx, nil); err != nil {
		return err
	}
	return nil
}
