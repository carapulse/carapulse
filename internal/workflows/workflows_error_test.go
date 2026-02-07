package workflows

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"carapulse/internal/tools"
)

func writeCLIWithScript(t *testing.T, dir, name, script, scriptWin string) {
	if runtime.GOOS == "windows" {
		name += ".bat"
		script = "@echo off\r\n" + scriptWin + "\r\n"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func withTempPath(t *testing.T, dir string) func() {
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	return func() { _ = os.Setenv("PATH", oldPath) }
}

func TestGitOpsDeployWorkflowArgoSyncError(t *testing.T) {
	tmp := t.TempDir()
	writeCLIWithScript(t, tmp, "argocd", "#!/bin/sh\nexit 1\n", "exit /b 1")
	defer withTempPath(t, tmp)()

	rt := NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})
	if err := GitOpsDeployWorkflow(context.Background(), DeployInput{PlanID: "p", ArgoCDApp: "app"}, rt, approveDB{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGitOpsDeployWorkflowArgoWaitError(t *testing.T) {
	tmp := t.TempDir()
	script := "#!/bin/sh\nif [ \"$2\" = \"wait\" ]; then exit 1; fi\nexit 0\n"
	scriptWin := "if \"%2\"==\"wait\" exit /b 1\r\nexit /b 0"
	writeCLIWithScript(t, tmp, "argocd", script, scriptWin)
	defer withTempPath(t, tmp)()

	rt := NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})
	if err := GitOpsDeployWorkflow(context.Background(), DeployInput{PlanID: "p", ArgoCDApp: "app"}, rt, approveDB{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestHelmReleaseWorkflowUpgradeError(t *testing.T) {
	tmp := t.TempDir()
	writeCLIWithScript(t, tmp, "helm", "#!/bin/sh\nexit 1\n", "exit /b 1")
	defer withTempPath(t, tmp)()

	rt := NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})
	if err := HelmReleaseWorkflow(context.Background(), HelmInput{PlanID: "p", Release: "rel", Strategy: "upgrade"}, rt, approveDB{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestHelmReleaseWorkflowRollbackError(t *testing.T) {
	tmp := t.TempDir()
	writeCLIWithScript(t, tmp, "helm", "#!/bin/sh\nexit 1\n", "exit /b 1")
	defer withTempPath(t, tmp)()

	rt := NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})
	if err := HelmReleaseWorkflow(context.Background(), HelmInput{PlanID: "p", Release: "rel", Strategy: "rollback"}, rt, approveDB{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestScaleServiceWorkflowScaleError(t *testing.T) {
	tmp := t.TempDir()
	writeCLIWithScript(t, tmp, "kubectl", "#!/bin/sh\nexit 1\n", "exit /b 1")
	defer withTempPath(t, tmp)()

	rt := NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})
	if err := ScaleServiceWorkflow(context.Background(), ScaleInput{PlanID: "p", Service: "deploy/app", Replicas: 1}, rt, approveDB{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestScaleServiceWorkflowRolloutError(t *testing.T) {
	tmp := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = \"rollout\" ]; then exit 1; fi\nexit 0\n"
	scriptWin := "if \"%1\"==\"rollout\" exit /b 1\r\nexit /b 0"
	writeCLIWithScript(t, tmp, "kubectl", script, scriptWin)
	defer withTempPath(t, tmp)()

	rt := NewRuntime(tools.NewRouter(), tools.NewSandbox(), tools.HTTPClients{})
	if err := ScaleServiceWorkflow(context.Background(), ScaleInput{PlanID: "p", Service: "deploy/app", Replicas: 1}, rt, approveDB{}); err == nil {
		t.Fatalf("expected error")
	}
}
