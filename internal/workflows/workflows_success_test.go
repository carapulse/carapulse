package workflows

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"carapulse/internal/tools"
)

type approveDB struct{}

func (a approveDB) GetApprovalStatus(ctx context.Context, planID string) (string, error) {
	return "approved", nil
}

func writeCLI(t *testing.T, dir, name string) {
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nexit /b 0\r\n"
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestWorkflowSuccessPaths(t *testing.T) {
	tmp := t.TempDir()
	writeCLI(t, tmp, "argocd")
	writeCLI(t, tmp, "helm")
	writeCLI(t, tmp, "kubectl")
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() { _ = os.Setenv("PATH", oldPath) }()

	router := tools.NewRouter()
	sandbox := tools.NewSandbox()
	rt := NewRuntime(router, sandbox, tools.HTTPClients{})
	ctx := context.Background()
	if err := GitOpsDeployWorkflow(ctx, DeployInput{PlanID: "p", ArgoCDApp: "app"}, rt, approveDB{}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if err := HelmReleaseWorkflow(ctx, HelmInput{PlanID: "p", Release: "rel", Strategy: "upgrade"}, rt, approveDB{}); err != nil {
		t.Fatalf("helm: %v", err)
	}
	if err := ScaleServiceWorkflow(ctx, ScaleInput{PlanID: "p", Service: "deploy/app", Replicas: 1}, rt, approveDB{}); err != nil {
		t.Fatalf("scale: %v", err)
	}
	if err := HelmReleaseWorkflow(ctx, HelmInput{PlanID: "p", Release: "rel", Strategy: "rollback"}, rt, approveDB{}); err != nil {
		t.Fatalf("helm rollback: %v", err)
	}
	if err := IncidentRemediationWorkflow(ctx, IncidentInput{PlanID: "p"}, rt, approveDB{}); err != nil {
		t.Fatalf("incident: %v", err)
	}
	if err := SecretRotationWorkflow(ctx, SecretRotationInput{PlanID: "p"}, rt, approveDB{}); err != nil {
		t.Fatalf("secret: %v", err)
	}
}
