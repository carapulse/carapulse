package workflows

import (
	"context"
	"errors"

	"carapulse/internal/web"
	"go.temporal.io/sdk/client"
)

type TemporalStarter struct {
	Client    client.Client
	TaskQueue string
}

func (s *TemporalStarter) StartExecution(ctx context.Context, planID, executionID string, ctxRef web.ContextRef, steps []web.PlanStep) (string, error) {
	if s == nil || s.Client == nil {
		return "", errors.New("temporal client required")
	}
	if executionID == "" {
		return "", errors.New("execution_id required")
	}
	input := PlanExecutionInput{
		PlanID:      planID,
		ExecutionID: executionID,
		Context: ContextRef{
			TenantID:      ctxRef.TenantID,
			Environment:   ctxRef.Environment,
			ClusterID:     ctxRef.ClusterID,
			Namespace:     ctxRef.Namespace,
			AWSAccountID:  ctxRef.AWSAccountID,
			Region:        ctxRef.Region,
			ArgoCDProject: ctxRef.ArgoCDProject,
			GrafanaOrgID:  ctxRef.GrafanaOrgID,
		},
		Steps: convertPlanSteps(steps),
	}
	opts := client.StartWorkflowOptions{
		ID:        "exec-" + executionID,
		TaskQueue: s.TaskQueue,
	}
	run, err := s.Client.ExecuteWorkflow(ctx, opts, PlanExecutionWorkflow, input)
	if err != nil {
		return "", err
	}
	return run.GetID(), nil
}

func convertPlanSteps(steps []web.PlanStep) []PlanStep {
	if len(steps) == 0 {
		return nil
	}
	out := make([]PlanStep, 0, len(steps))
	for _, step := range steps {
		out = append(out, PlanStep{
			StepID:        step.StepID,
			Stage:         step.Stage,
			Action:        step.Action,
			Tool:          step.Tool,
			Input:         step.Input,
			Preconditions: step.Preconditions,
			Rollback:      step.Rollback,
		})
	}
	return out
}
