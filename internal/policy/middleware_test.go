package policy

import (
	"errors"
	"testing"
)

type fakeChecker struct {}

func (f fakeChecker) Evaluate(input PolicyInput) (PolicyDecision, error) {
	return PolicyDecision{Decision: "deny"}, nil
}

type errorChecker struct {}

func (e errorChecker) Evaluate(input PolicyInput) (PolicyDecision, error) {
	return PolicyDecision{}, ErrTestPolicy
}

var ErrTestPolicy = errors.New("policy error")

func TestEvaluatorDefaultAllow(t *testing.T) {
	e := Evaluator{}
	dec, err := e.Check(nil, PolicyInput{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if dec.Decision != "allow" {
		t.Fatalf("decision: %s", dec.Decision)
	}
}

func TestEvaluatorUsesChecker(t *testing.T) {
	e := Evaluator{Checker: fakeChecker{}}
	dec, err := e.Check(nil, PolicyInput{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if dec.Decision != "deny" {
		t.Fatalf("decision: %s", dec.Decision)
	}
}

func TestEvaluatorError(t *testing.T) {
	e := Evaluator{Checker: errorChecker{}}
	if _, err := e.Check(nil, PolicyInput{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCheckerFunc(t *testing.T) {
	called := false
	fn := CheckerFunc(func(input PolicyInput) (PolicyDecision, error) {
		called = true
		return PolicyDecision{Decision: "allow"}, nil
	})
	if _, err := fn.Evaluate(PolicyInput{}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !called {
		t.Fatalf("expected call")
	}
}
