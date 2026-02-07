package web

import (
	"context"
	"errors"
	"testing"

	"carapulse/internal/policy"
)

type diagStub struct {
	items []DiagnosticEvidence
}

func (d diagStub) Collect(ctx context.Context, ctxRef ContextRef, intent string, constraints any) ([]DiagnosticEvidence, error) {
	return d.items, nil
}

type plannerStub struct {
	text string
}

func (p plannerStub) Plan(intent string, context any, evidence any) (string, error) {
	return p.text, nil
}

type execStub struct {
	calls int
}

func (e *execStub) StartExecution(ctx context.Context, planID, executionID string, ctxRef ContextRef, steps []PlanStep) (string, error) {
	e.calls++
	return "wf_1", nil
}

type approvalDB struct {
	fakeDB
	created int
}

func (a *approvalDB) CreateApproval(ctx context.Context, planID string, payload []byte) (string, error) {
	a.created++
	return "approval_1", nil
}

func validHookPayload() map[string]any {
	return map[string]any{
		"context": map[string]any{
			"tenant_id":      "t",
			"environment":    "dev",
			"cluster_id":     "c",
			"namespace":      "ns",
			"aws_account_id": "a",
			"region":         "r",
			"argocd_project": "p",
			"grafana_org_id": "g",
		},
	}
}

func TestRunAlertEventLoopDBRequired(t *testing.T) {
	s := &Server{}
	if _, err := s.runAlertEventLoop(context.Background(), "alertmanager", validHookPayload()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunAlertEventLoopInvalidContext(t *testing.T) {
	s := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) {
		return policy.PolicyDecision{Decision: "allow"}, nil
	})}}
	if _, err := s.runAlertEventLoop(context.Background(), "alertmanager", map[string]any{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunAlertEventLoopRequireApprovalRead(t *testing.T) {
	s := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) {
		return policy.PolicyDecision{Decision: "require_approval"}, nil
	})}}
	if _, err := s.runAlertEventLoop(context.Background(), "alertmanager", validHookPayload()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunAlertEventLoopPolicyDeny(t *testing.T) {
	s := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) {
		return policy.PolicyDecision{Decision: "deny"}, nil
	})}}
	payload := validHookPayload()
	payload["alerts"] = []any{map[string]any{"labels": map[string]any{"alertname": "deploy"}}}
	if _, err := s.runAlertEventLoop(context.Background(), "alertmanager", payload); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunAlertEventLoopWriteHappyPath(t *testing.T) {
	db := &fakeDB{}
	exec := &execStub{}
	s := &Server{
		DB:              db,
		Policy:          &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) { return policy.PolicyDecision{Decision: "allow"}, nil })},
		Diagnostics:     diagStub{items: []DiagnosticEvidence{{Type: "promql"}}},
		Planner:         plannerStub{text: "1. step one\n2. step two"},
		Executor:        exec,
		AutoApproveLow:  true,
		EnableEventLoop: true,
	}
	payload := validHookPayload()
	payload["alerts"] = []any{map[string]any{"labels": map[string]any{"alertname": "deploy service"}}}
	res, err := s.runAlertEventLoop(context.Background(), "alertmanager", payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.PlanID == "" || res.ExecutionID == "" {
		t.Fatalf("res: %#v", res)
	}
	if db.updateStatus != "approved" {
		t.Fatalf("status: %s", db.updateStatus)
	}
	if exec.calls != 1 {
		t.Fatalf("exec calls: %d", exec.calls)
	}
}

func TestRunAlertEventLoopRequireApprovalWrite(t *testing.T) {
	db := &approvalDB{}
	s := &Server{
		DB:     db,
		Policy: &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) { return policy.PolicyDecision{Decision: "require_approval"}, nil })},
		Planner: plannerStub{text: "1. step"},
	}
	payload := validHookPayload()
	payload["alerts"] = []any{map[string]any{"labels": map[string]any{"alertname": "scale service"}}}
	res, err := s.runAlertEventLoop(context.Background(), "alertmanager", payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.PlanID == "" {
		t.Fatalf("missing plan id")
	}
	if db.created == 0 {
		t.Fatalf("expected approval create")
	}
}

func TestRunAlertEventLoopPlannerError(t *testing.T) {
	s := &Server{
		DB:     &fakeDB{},
		Policy: &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) { return policy.PolicyDecision{Decision: "allow"}, nil })},
	}
	s.Planner = plannerStub{text: ""}
	payload := validHookPayload()
	payload["summary"] = "scale"
	old := marshalJSON
	defer func() { marshalJSON = old }()
	marshalJSON = func(v any) ([]byte, error) { return nil, errors.New("marshal") }
	if _, err := s.runAlertEventLoop(context.Background(), "alertmanager", payload); err == nil {
		t.Fatalf("expected error")
	}
}
