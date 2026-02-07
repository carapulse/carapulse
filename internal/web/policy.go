package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"carapulse/internal/policy"
)

func (s *Server) policyDecision(r *http.Request, actionName string, actionType string, ctxRef ContextRef, risk string, targets int) (policy.PolicyDecision, error) {
	actor, _ := ActorFromContext(r.Context())
	tier := tierForRisk(risk)
	blast := blastRadius(ctxRef, targets)
	breakGlass := strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Break-Glass")), "true")
	dec := policy.PolicyDecision{Decision: "allow"}
	if s.Policy == nil || s.Policy.Checker == nil {
		if actionType == "read" && s.FailOpenReads {
			return policy.PolicyDecision{Decision: "allow"}, nil
		}
		return policy.PolicyDecision{}, errors.New("policy required")
	}
	if s.Policy != nil {
		var err error
		dec, err = s.Policy.Check(r.Context(), policy.PolicyInput{
			Actor:     actor,
			Action:    policy.Action{Name: actionName, Type: actionType},
			Context:   ctxRef,
			Risk:      policy.Risk{Level: risk, Targets: targets, BlastRadius: blast, Tier: tier},
			Time:      time.Now().UTC().Format(time.RFC3339),
			Resources: map[string]any{"break_glass": breakGlass},
		})
		if err != nil {
			if actionType == "read" {
				return policy.PolicyDecision{Decision: "allow"}, nil
			}
			return policy.PolicyDecision{}, err
		}
		if dec.Decision == "" {
			dec.Decision = "allow"
		}
	}
	if actionType == "write" && strings.EqualFold(strings.TrimSpace(ctxRef.Environment), "prod") && dec.Decision == "allow" {
		dec.Decision = "require_approval"
	}
	if actionType == "write" && dec.Decision == "allow" && risk != "read" && risk != "low" {
		dec.Decision = "require_approval"
	}
	if actionType == "write" && blast == "account" {
		dec.Decision = "require_approval"
	}
	if actionType == "write" && tier == "break_glass" && !breakGlass {
		dec.Decision = "deny"
	}
	return dec, nil
}

func (s *Server) policyCheck(r *http.Request, actionName string, actionType string, ctxRef ContextRef, risk string, targets int) error {
	dec, err := s.policyDecision(r, actionName, actionType, ctxRef, risk, targets)
	if err != nil {
		return err
	}
	if dec.Decision != "allow" {
		return errors.New("policy decision " + dec.Decision)
	}
	return nil
}
