package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func GitOpsDeployWorkflowTemporal(ctx workflow.Context, in DeployInput) error {
	input := map[string]any{
		"service":    in.Service,
		"argocd_app": in.ArgoCDApp,
		"revision":   in.Revision,
	}
	_, steps, err := BuildWorkflowSteps("gitops_deploy", input)
	if err != nil {
		return err
	}
	return runWorkflowSteps(ctx, in.PlanID, in.Context, steps)
}

func HelmReleaseWorkflowTemporal(ctx workflow.Context, in HelmInput) error {
	input := map[string]any{
		"release":    in.Release,
		"chart":      in.Chart,
		"values_ref": in.ValuesRef,
		"namespace":  in.Namespace,
		"strategy":   in.Strategy,
	}
	_, steps, err := BuildWorkflowSteps("helm_release", input)
	if err != nil {
		return err
	}
	return runWorkflowSteps(ctx, in.PlanID, in.Context, steps)
}

func ScaleServiceWorkflowTemporal(ctx workflow.Context, in ScaleInput) error {
	input := map[string]any{
		"service":  in.Service,
		"replicas": in.Replicas,
	}
	_, steps, err := BuildWorkflowSteps("scale_service", input)
	if err != nil {
		return err
	}
	return runWorkflowSteps(ctx, in.PlanID, in.Context, steps)
}

func IncidentRemediationWorkflowTemporal(ctx workflow.Context, in IncidentInput) error {
	input := map[string]any{
		"alert_id": in.AlertID,
		"service":  in.Service,
	}
	_, steps, err := BuildWorkflowSteps("incident_remediation", input)
	if err != nil {
		return err
	}
	return runWorkflowSteps(ctx, in.PlanID, in.Context, steps)
}

func SecretRotationWorkflowTemporal(ctx workflow.Context, in SecretRotationInput) error {
	input := map[string]any{
		"secret_path": in.SecretPath,
		"target":      in.Target,
	}
	_, steps, err := BuildWorkflowSteps("secret_rotation", input)
	if err != nil {
		return err
	}
	return runWorkflowSteps(ctx, in.PlanID, in.Context, steps)
}

func runWorkflowSteps(ctx workflow.Context, planID string, ctxRef ContextRef, steps []PlanStep) error {
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
	if err := workflow.ExecuteActivity(ctx, "CheckApproval", planID).Get(ctx, nil); err != nil {
		return err
	}
	var executionID string
	if err := workflow.ExecuteActivity(ctx, "CreateExecution", planID).Get(ctx, &executionID); err != nil {
		return err
	}
	if err := workflow.ExecuteActivity(ctx, "UpdateExecutionStatus", executionID, "running").Get(ctx, nil); err != nil {
		return err
	}
	actSteps, verifySteps := splitStepsByStage(steps)
	for _, step := range actSteps {
		actInput := StepActivityInput{
			PlanID:      planID,
			ExecutionID: executionID,
			Context:     ctxRef,
			Step:        step,
		}
		if err := workflow.ExecuteActivity(ctx, "ExecuteStep", actInput).Get(ctx, nil); err != nil {
			_ = workflow.ExecuteActivity(ctx, "RollbackStep", actInput).Get(ctx, nil)
			_ = workflow.ExecuteActivity(ctx, "CompleteExecution", executionID, "failed").Get(ctx, nil)
			return err
		}
	}
	for _, step := range verifySteps {
		verifyInput := StepActivityInput{
			PlanID:      planID,
			ExecutionID: executionID,
			Context:     ctxRef,
			Step:        step,
		}
		if err := workflow.ExecuteActivity(ctx, "ExecuteStep", verifyInput).Get(ctx, nil); err != nil {
			for i := len(actSteps) - 1; i >= 0; i-- {
				rollbackInput := StepActivityInput{
					PlanID:      planID,
					ExecutionID: executionID,
					Context:     ctxRef,
					Step:        actSteps[i],
				}
				_ = workflow.ExecuteActivity(ctx, "RollbackStep", rollbackInput).Get(ctx, nil)
			}
			_ = workflow.ExecuteActivity(ctx, "CompleteExecution", executionID, "failed").Get(ctx, nil)
			return err
		}
	}
	return workflow.ExecuteActivity(ctx, "CompleteExecution", executionID, "succeeded").Get(ctx, nil)
}
