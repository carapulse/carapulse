package web

import "testing"

func TestIsWriteAction(t *testing.T) {
	cases := []struct {
		intent string
		want   bool
	}{
		{"deploy service", true},
		{"scale api", true},
		{"rollback release", true},
		{"sync argo", true},
		{"delete user", true},
		{"show status", false},
	}
	for _, c := range cases {
		got := isWriteAction(c.intent)
		if got != c.want {
			t.Fatalf("intent %q: want %v got %v", c.intent, c.want, got)
		}
	}
}

func TestRiskFromIntent(t *testing.T) {
	cases := []struct {
		intent string
		want   string
	}{
		{"deploy service", "low"},
		{"scale api", "low"},
		{"rollback release", "low"},
		{"sync argo", "medium"},
		{"restart deployment", "medium"},
		{"delete user", "high"},
		{"update iam role", "high"},
		{"show status", "read"},
	}
	for _, c := range cases {
		got := riskFromIntent(c.intent)
		if got != c.want {
			t.Fatalf("intent %q: want %s got %s", c.intent, c.want, got)
		}
	}
}

func TestRiskFromSteps(t *testing.T) {
	cases := []struct {
		name  string
		steps []PlanStep
		want  string
	}{
		{"empty", nil, "read"},
		{"read only", []PlanStep{{Tool: "prometheus", Action: "query"}}, "read"},
		{"kubectl get", []PlanStep{{Tool: "kubectl", Action: "get"}}, "read"},
		{"kubectl delete is high", []PlanStep{{Tool: "kubectl", Action: "delete"}}, "high"},
		{"kubectl exec is high", []PlanStep{{Tool: "kubectl", Action: "exec"}}, "high"},
		{"helm upgrade is medium", []PlanStep{{Tool: "helm", Action: "upgrade"}}, "medium"},
		{"helm uninstall is high", []PlanStep{{Tool: "helm", Action: "uninstall"}}, "high"},
		{"argocd sync is medium", []PlanStep{{Tool: "argocd", Action: "sync"}}, "medium"},
		{"argocd delete is high", []PlanStep{{Tool: "argocd", Action: "delete"}}, "high"},
		{"aws terminate is high", []PlanStep{{Tool: "aws", Action: "terminate-instances"}}, "high"},
		{"mixed read+high returns high", []PlanStep{
			{Tool: "prometheus", Action: "query"},
			{Tool: "kubectl", Action: "delete"},
		}, "high"},
		{"mixed medium+low returns medium", []PlanStep{
			{Tool: "helm", Action: "upgrade"},
			{Tool: "grafana", Action: "annotate"},
		}, "medium"},
	}
	for _, c := range cases {
		got := riskFromSteps(c.steps)
		if got != c.want {
			t.Fatalf("%s: want %s got %s", c.name, c.want, got)
		}
	}
}

func TestEffectiveRiskEscalatesOnly(t *testing.T) {
	// Intent says "low" but steps contain kubectl delete -> should escalate to high.
	steps := []PlanStep{{Tool: "kubectl", Action: "delete"}}
	got := effectiveRisk("low", steps)
	if got != "high" {
		t.Fatalf("want high, got %s", got)
	}

	// Intent says "high" but steps are read-only -> should stay high (never downgrade).
	readSteps := []PlanStep{{Tool: "prometheus", Action: "query"}}
	got = effectiveRisk("high", readSteps)
	if got != "high" {
		t.Fatalf("want high, got %s", got)
	}
}

func TestRiskFromStepDrafts(t *testing.T) {
	drafts := []planStepDraft{
		{Tool: "kubectl", Action: "apply"},
		{Tool: "prometheus", Action: "query"},
	}
	got := riskFromStepDrafts(drafts)
	if got != "high" {
		t.Fatalf("want high, got %s", got)
	}
}

func TestEffectiveRiskDraftsEscalates(t *testing.T) {
	drafts := []planStepDraft{{Tool: "helm", Action: "uninstall"}}
	got := effectiveRiskDrafts("low", drafts)
	if got != "high" {
		t.Fatalf("want high, got %s", got)
	}
}

func TestRiskOrd(t *testing.T) {
	if riskOrd("read") >= riskOrd("low") {
		t.Fatalf("read should be less than low")
	}
	if riskOrd("low") >= riskOrd("medium") {
		t.Fatalf("low should be less than medium")
	}
	if riskOrd("medium") >= riskOrd("high") {
		t.Fatalf("medium should be less than high")
	}
}
