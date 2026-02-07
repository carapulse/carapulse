package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type EventLoopResult struct {
	PlanID      string `json:"plan_id"`
	ExecutionID string `json:"execution_id"`
}

func (s *Server) ShouldRunEventLoop(source string) bool {
	return s.shouldRunEventLoop(source)
}

func (s *Server) RunAlertEventLoop(ctx context.Context, source string, payload map[string]any) (EventLoopResult, error) {
	return s.runAlertEventLoop(ctx, source, payload)
}

func (s *Server) runAlertEventLoop(ctx context.Context, source string, payload map[string]any) (EventLoopResult, error) {
	if s.DB == nil {
		return EventLoopResult{}, errors.New("db unavailable")
	}
	ctxRef := contextFromHook(payload)
	if err := validateContextRefStrict(ctxRef); err != nil {
		return EventLoopResult{}, err
	}
	summary := hookSummary(source, payload)
	intent := summary
	risk := riskFromIntent(intent)
	actionType := "read"
	if risk != "read" {
		actionType = "write"
	}
	req := (&http.Request{Header: http.Header{}}).WithContext(ctx)
	dec, err := s.policyDecision(req, "plan.create", actionType, ctxRef, risk, 0)
	if err != nil {
		return EventLoopResult{}, err
	}
	switch dec.Decision {
	case "allow":
	case "require_approval":
		if actionType != "write" {
			return EventLoopResult{}, errors.New("approval required")
		}
	default:
		return EventLoopResult{}, errors.New("policy decision " + dec.Decision)
	}
	mergedConstraints := mergeConstraints(nil, dec.Constraints)
	var serviceGraph any
	if s.Context != nil {
		if service := serviceFromHook(payload); service != "" {
			if graph, err := s.Context.GetServiceGraph(ctx, service); err == nil && (len(graph.Nodes) > 0 || len(graph.Edges) > 0) {
				serviceGraph = graph
			}
		}
	}
	if hints := diagnosticHintsFromGraph(serviceGraph); len(hints) > 0 {
		mergedConstraints = mergeConstraints(mergedConstraints, map[string]any{"diagnostic_hints": hints})
	}
	var diagnostics []DiagnosticEvidence
	if s.Diagnostics != nil {
		if collected, err := s.Diagnostics.Collect(ctx, ctxRef, intent, mergedConstraints); err == nil {
			diagnostics = collected
		}
	}
	createdAt := time.Now().UTC()
	var planText string
	if s.Planner != nil && intent != "" {
		planContext := map[string]any{
			"context":     ctxRef,
			"summary":     summary,
			"source":      source,
			"payload":     payload,
			"trigger":     "alert",
			"diagnostics": diagnostics,
		}
		if serviceGraph != nil {
			planContext["service_graph"] = serviceGraph
		}
		draft, err := s.Planner.Plan(intent, planContext, diagnostics)
		if err != nil {
			return EventLoopResult{}, err
		}
		planText = draft
	}
	plan := map[string]any{
		"trigger":     "alert",
		"summary":     summary,
		"context":     ctxRef,
		"risk_level":  risk,
		"intent":      intent,
		"constraints": mergedConstraints,
		"created_at":  createdAt,
	}
	var stepsDraft []planStepDraft
	if planText != "" {
		plan["plan_text"] = planText
		stepsDraft = parsePlanSteps(planText)
		if len(stepsDraft) > 0 {
			if actionType == "write" {
				stepsDraft = ensureVerifySteps(stepsDraft, diagnostics, intent)
			}
			plan["steps"] = stepsDraft
		}
	}
	if actionType == "write" && len(stepsDraft) == 0 {
		if defaults := defaultVerifySteps(diagnostics, intent); len(defaults) > 0 {
			stepsDraft = defaults
			plan["steps"] = defaults
		}
	}
	if len(diagnostics) > 0 {
		if data, err := json.Marshal(diagnostics); err == nil {
			plan["diagnostics"] = json.RawMessage(data)
		}
	}
	if serviceGraph != nil {
		if meta, ok := plan["meta"].(map[string]any); ok {
			meta["service_graph"] = serviceGraph
		} else {
			plan["meta"] = map[string]any{"service_graph": serviceGraph}
		}
	}
	data, err := marshalJSON(plan)
	if err != nil {
		return EventLoopResult{}, err
	}
	planID, err := s.DB.CreatePlan(ctx, data)
	if err != nil {
		return EventLoopResult{}, err
	}
	execID := ""
	if actionType == "write" {
		if risk == "low" && s.AutoApproveLow && dec.Decision != "require_approval" {
			if _, err := s.createApproval(ctx, planID, false); err == nil {
				_ = s.DB.UpdateApprovalStatusByPlan(ctx, planID, "approved")
			}
		} else {
			if _, err := s.createApproval(ctx, planID, true); err != nil {
				return EventLoopResult{}, err
			}
		}
		if s.Executor != nil {
			execID, err = s.DB.CreateExecution(ctx, planID)
			if err != nil {
				return EventLoopResult{}, err
			}
			steps := []PlanStep{}
			if raw, err := json.Marshal(plan["steps"]); err == nil {
				if parsed, err := parsePlanStepsJSON(raw); err == nil {
					steps = parsed
				}
			}
			if _, err := s.Executor.StartExecution(ctx, planID, execID, ctxRef, steps); err != nil {
				return EventLoopResult{}, err
			}
		}
	}
	return EventLoopResult{PlanID: planID, ExecutionID: execID}, nil
}
