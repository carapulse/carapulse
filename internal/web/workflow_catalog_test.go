package web

import "testing"

func TestWorkflowCatalogLookup(t *testing.T) {
	if _, ok := findWorkflowTemplate("gitops_deploy"); !ok {
		t.Fatalf("expected template")
	}
	if _, ok := findWorkflowTemplate("missing"); ok {
		t.Fatalf("unexpected template")
	}
}

func TestBuildWorkflowPlanErrors(t *testing.T) {
	if _, _, err := buildWorkflowPlan("missing", nil); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, err := buildGitOpsDeploy(map[string]any{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, err := buildHelmRelease(map[string]any{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, err := buildScaleService(map[string]any{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, err := buildSecretRotation(map[string]any{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildWorkflowPlans(t *testing.T) {
	_, steps, err := buildGitOpsDeploy(map[string]any{"argocd_app": "app", "service": "svc", "revision": "r1", "promql": "up"})
	if err != nil || len(steps) == 0 {
		t.Fatalf("gitops err=%v steps=%d", err, len(steps))
	}
	_, steps, err = buildHelmRelease(map[string]any{"release": "rel", "strategy": "rollback"})
	if err != nil || len(steps) == 0 {
		t.Fatalf("helm err=%v steps=%d", err, len(steps))
	}
	_, steps, err = buildHelmRelease(map[string]any{"release": "rel", "chart": "c", "values_ref": "v"})
	if err != nil || len(steps) == 0 {
		t.Fatalf("helm upgrade err=%v steps=%d", err, len(steps))
	}
	_, steps, err = buildScaleService(map[string]any{"resource": "deploy/app", "replicas": 2, "previous_replicas": 1})
	if err != nil || len(steps) == 0 {
		t.Fatalf("scale err=%v steps=%d", err, len(steps))
	}
	_, steps, err = buildIncidentRemediation(map[string]any{
		"service": "svc",
		"remediation": map[string]any{
			"tool":   "kubectl",
			"action": "rollout",
			"input":  map[string]any{"resource": "deploy/app"},
		},
	})
	if err != nil || len(steps) == 0 {
		t.Fatalf("incident err=%v steps=%d", err, len(steps))
	}
	_, steps, err = buildSecretRotation(map[string]any{"secret_path": "lease", "argocd_app": "app"})
	if err != nil || len(steps) == 0 {
		t.Fatalf("secret err=%v steps=%d", err, len(steps))
	}
}

func TestWorkflowHelpers(t *testing.T) {
	if got := stringValue(map[string]any{"a": "1"}, "a"); got != "1" {
		t.Fatalf("string: %s", got)
	}
	if _, ok := intValue(map[string]any{"a": "1"}, "a"); ok {
		t.Fatalf("expected int fail")
	}
	if v, ok := intValue(map[string]any{"a": float64(2)}, "a"); !ok || v != 2 {
		t.Fatalf("int: %d ok=%v", v, ok)
	}
	if rollbackStep("tool", "action", map[string]any{"k": "v"}, false) != nil {
		t.Fatalf("expected nil rollback")
	}
}
