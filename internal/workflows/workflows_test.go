package workflows

import (
	"context"
	"testing"
)

type denyDB struct{}

func (d denyDB) GetApprovalStatus(ctx context.Context, planID string) (string, error) {
	return "denied", nil
}

func TestWorkflowsRequireApproval(t *testing.T) {
	rt := &Runtime{}
	if err := GitOpsDeployWorkflow(context.Background(), DeployInput{PlanID: "p"}, rt, denyDB{}); err == nil {
		t.Fatalf("expected error")
	}
	if err := HelmReleaseWorkflow(context.Background(), HelmInput{PlanID: "p"}, rt, denyDB{}); err == nil {
		t.Fatalf("expected error")
	}
	if err := ScaleServiceWorkflow(context.Background(), ScaleInput{PlanID: "p"}, rt, denyDB{}); err == nil {
		t.Fatalf("expected error")
	}
	if err := IncidentRemediationWorkflow(context.Background(), IncidentInput{PlanID: "p"}, rt, denyDB{}); err == nil {
		t.Fatalf("expected error")
	}
	if err := SecretRotationWorkflow(context.Background(), SecretRotationInput{PlanID: "p"}, rt, denyDB{}); err == nil {
		t.Fatalf("expected error")
	}
}
