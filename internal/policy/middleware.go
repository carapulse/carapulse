package policy

import "context"

type Checker interface {
	Evaluate(input PolicyInput) (PolicyDecision, error)
}

type CheckerFunc func(input PolicyInput) (PolicyDecision, error)

func (f CheckerFunc) Evaluate(input PolicyInput) (PolicyDecision, error) {
	return f(input)
}

type Evaluator struct {
	Checker Checker
}

func (e *Evaluator) Check(ctx context.Context, input PolicyInput) (PolicyDecision, error) {
	if e.Checker == nil {
		return PolicyDecision{Decision: "allow"}, nil
	}
	return e.Checker.Evaluate(input)
}
