package workflows

import (
	"context"
	"errors"
	"time"

	"carapulse/internal/tools"
)

type Activities struct {
	Store      ExecutionStore
	Runtime    *Runtime
	Objects    BlobStore
	PresignTTL time.Duration
}

func (a *Activities) executor() *Executor {
	return &Executor{
		Store:      a.Store,
		Runtime:    a.Runtime,
		Objects:    a.Objects,
		PresignTTL: a.PresignTTL,
	}
}

func (a *Activities) ExecuteStep(ctx context.Context, input StepActivityInput) error {
	if a.Store == nil || a.Runtime == nil {
		return errors.New("runtime required")
	}
	exec := a.executor()
	return exec.executeStep(ctx, input.ExecutionID, input.Step, contextToTools(input.Context))
}

func (a *Activities) RollbackStep(ctx context.Context, input StepActivityInput) error {
	if a.Store == nil || a.Runtime == nil {
		return errors.New("runtime required")
	}
	exec := a.executor()
	return exec.tryRollback(ctx, input.ExecutionID, input.Step, contextToTools(input.Context))
}

func (a *Activities) UpdateExecutionStatus(ctx context.Context, executionID, status string) error {
	if a.Store == nil {
		return errors.New("store required")
	}
	return a.Store.UpdateExecutionStatus(ctx, executionID, status)
}

func (a *Activities) CompleteExecution(ctx context.Context, executionID, status string) error {
	if a.Store == nil {
		return errors.New("store required")
	}
	return a.Store.CompleteExecution(ctx, executionID, status)
}

func (a *Activities) CheckApproval(ctx context.Context, planID string) error {
	if a.Store == nil {
		return ErrApprovalRequired
	}
	reader, ok := any(a.Store).(DBReader)
	if !ok {
		return ErrApprovalRequired
	}
	return RequireApproval(ctx, reader, planID)
}

func (a *Activities) CreateExecution(ctx context.Context, planID string) (string, error) {
	if a.Store == nil {
		return "", errors.New("store required")
	}
	writer, ok := any(a.Store).(interface {
		CreateExecution(ctx context.Context, planID string) (string, error)
	})
	if !ok {
		return "", errors.New("create execution unsupported")
	}
	return writer.CreateExecution(ctx, planID)
}

func contextToTools(ctxRef ContextRef) tools.ContextRef {
	return tools.ContextRef{
		TenantID:      ctxRef.TenantID,
		Environment:   ctxRef.Environment,
		ClusterID:     ctxRef.ClusterID,
		Namespace:     ctxRef.Namespace,
		AWSAccountID:  ctxRef.AWSAccountID,
		Region:        ctxRef.Region,
		ArgoCDProject: ctxRef.ArgoCDProject,
		GrafanaOrgID:  ctxRef.GrafanaOrgID,
	}
}
