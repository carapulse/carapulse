package workflows

import "testing"

func TestBuildWorkflowStepsErrors(t *testing.T) {
	if _, _, err := BuildWorkflowSteps("missing", nil); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, err := BuildWorkflowSteps("gitops_deploy", map[string]any{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, err := BuildWorkflowSteps("helm_release", map[string]any{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, err := BuildWorkflowSteps("scale_service", map[string]any{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, err := BuildWorkflowSteps("secret_rotation", map[string]any{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildWorkflowStepsSuccess(t *testing.T) {
	_, steps, err := BuildWorkflowSteps("gitops_deploy", map[string]any{"argocd_app": "app", "service": "svc", "promql": "up"})
	if err != nil || len(steps) == 0 {
		t.Fatalf("gitops err=%v steps=%d", err, len(steps))
	}
	_, steps, err = BuildWorkflowSteps("helm_release", map[string]any{"release": "rel", "strategy": "rollback"})
	if err != nil || len(steps) == 0 {
		t.Fatalf("helm err=%v steps=%d", err, len(steps))
	}
	_, steps, err = BuildWorkflowSteps("scale_service", map[string]any{"resource": "deploy/app", "replicas": 2, "previous_replicas": 1})
	if err != nil || len(steps) == 0 {
		t.Fatalf("scale err=%v steps=%d", err, len(steps))
	}
	_, steps, err = BuildWorkflowSteps("incident_remediation", map[string]any{"service": "svc"})
	if err != nil || len(steps) == 0 {
		t.Fatalf("incident err=%v steps=%d", err, len(steps))
	}
	_, steps, err = BuildWorkflowSteps("secret_rotation", map[string]any{"secret_path": "lease"})
	if err != nil || len(steps) == 0 {
		t.Fatalf("secret err=%v steps=%d", err, len(steps))
	}
}

func TestWorkflowTemplateHelpers(t *testing.T) {
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
