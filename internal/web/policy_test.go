package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"carapulse/internal/policy"
)

type denyCheckerPolicy struct{}

func (d denyCheckerPolicy) Evaluate(input policy.PolicyInput) (policy.PolicyDecision, error) {
	return policy.PolicyDecision{Decision: "deny"}, nil
}

type errorCheckerPolicy struct{}

func (e errorCheckerPolicy) Evaluate(input policy.PolicyInput) (policy.PolicyDecision, error) {
	return policy.PolicyDecision{}, errTestPolicy
}

var errTestPolicy = errors.New("policy error")

type allowCheckerPolicy struct{}

func (a allowCheckerPolicy) Evaluate(input policy.PolicyInput) (policy.PolicyDecision, error) {
	return policy.PolicyDecision{Decision: "allow"}, nil
}

type emptyDecisionPolicy struct{}

func (e emptyDecisionPolicy) Evaluate(input policy.PolicyInput) (policy.PolicyDecision, error) {
	return policy.PolicyDecision{}, nil
}

type requireApprovalPolicy struct{}

func (r requireApprovalPolicy) Evaluate(input policy.PolicyInput) (policy.PolicyDecision, error) {
	return policy.PolicyDecision{Decision: "require_approval"}, nil
}

func TestPolicyCheckDeny(t *testing.T) {
	s := &Server{Policy: &policy.Evaluator{Checker: denyCheckerPolicy{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	dec, err := s.policyDecision(req, "plan.create", "write", ContextRef{}, "low", 0)
	if err != nil || dec.Decision != "deny" {
		t.Fatalf("decision: %#v err: %v", dec, err)
	}
}

func TestPolicyCheckNoPolicy(t *testing.T) {
	s := &Server{Policy: nil}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	dec, err := s.policyDecision(req, "plan.create", "write", ContextRef{}, "low", 0)
	if err != nil || dec.Decision != "allow" {
		t.Fatalf("decision: %#v err: %v", dec, err)
	}
}

func TestPolicyDecisionError(t *testing.T) {
	s := &Server{Policy: &policy.Evaluator{Checker: errorCheckerPolicy{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	if _, err := s.policyDecision(req, "plan.create", "write", ContextRef{}, "low", 0); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPolicyDecisionErrorReadAllows(t *testing.T) {
	s := &Server{Policy: &policy.Evaluator{Checker: errorCheckerPolicy{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	dec, err := s.policyDecision(req, "plan.create", "read", ContextRef{}, "low", 0)
	if err != nil || dec.Decision != "allow" {
		t.Fatalf("decision: %#v err: %v", dec, err)
	}
}

func TestPolicyDecisionAllow(t *testing.T) {
	s := &Server{Policy: &policy.Evaluator{Checker: allowCheckerPolicy{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	dec, err := s.policyDecision(req, "plan.create", "write", ContextRef{}, "low", 0)
	if err != nil || dec.Decision != "allow" {
		t.Fatalf("decision: %#v err: %v", dec, err)
	}
}

func TestPolicyDecisionProdRequiresApproval(t *testing.T) {
	s := &Server{Policy: &policy.Evaluator{Checker: allowCheckerPolicy{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	dec, err := s.policyDecision(req, "plan.create", "write", ContextRef{Environment: "prod"}, "low", 0)
	if err != nil || dec.Decision != "require_approval" {
		t.Fatalf("decision: %#v err: %v", dec, err)
	}
}

func TestPolicyDecisionRequireApproval(t *testing.T) {
	s := &Server{Policy: &policy.Evaluator{Checker: requireApprovalPolicy{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	dec, err := s.policyDecision(req, "plan.create", "write", ContextRef{}, "low", 0)
	if err != nil || dec.Decision != "require_approval" {
		t.Fatalf("decision: %#v err: %v", dec, err)
	}
}

func TestPolicyDecisionEmptyDefaultsAllow(t *testing.T) {
	s := &Server{Policy: &policy.Evaluator{Checker: emptyDecisionPolicy{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	dec, err := s.policyDecision(req, "plan.create", "write", ContextRef{}, "low", 0)
	if err != nil || dec.Decision != "allow" {
		t.Fatalf("decision: %#v err: %v", dec, err)
	}
}

func TestPolicyCheckRequireApproval(t *testing.T) {
	s := &Server{Policy: &policy.Evaluator{Checker: requireApprovalPolicy{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	if err := s.policyCheck(req, "plan.create", "write", ContextRef{}, "low", 0); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPolicyCheckAllow(t *testing.T) {
	s := &Server{Policy: &policy.Evaluator{Checker: allowCheckerPolicy{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	if err := s.policyCheck(req, "plan.create", "write", ContextRef{}, "low", 0); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestPolicyCheckDecisionError(t *testing.T) {
	s := &Server{Policy: &policy.Evaluator{Checker: errorCheckerPolicy{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	if err := s.policyCheck(req, "plan.create", "write", ContextRef{}, "low", 0); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPolicyDecisionBreakGlassRequired(t *testing.T) {
	s := &Server{Policy: &policy.Evaluator{Checker: allowCheckerPolicy{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	dec, err := s.policyDecision(req, "plan.execute", "write", ContextRef{}, "high", 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if dec.Decision != "deny" {
		t.Fatalf("expected deny, got %s", dec.Decision)
	}
}

func TestPolicyDecisionBreakGlassHeader(t *testing.T) {
	s := &Server{Policy: &policy.Evaluator{Checker: allowCheckerPolicy{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Break-Glass", "true")
	req = req.WithContext(WithActor(req.Context(), Actor{ID: "u"}))
	dec, err := s.policyDecision(req, "plan.execute", "write", ContextRef{}, "high", 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if dec.Decision != "require_approval" {
		t.Fatalf("expected approval, got %s", dec.Decision)
	}
}
