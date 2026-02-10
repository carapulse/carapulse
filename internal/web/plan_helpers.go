package web

import "strings"

func isWriteAction(intent string) bool {
	return riskFromIntent(intent) != "read"
}

func riskFromIntent(intent string) string {
	intent = strings.ToLower(intent)
	switch {
	case strings.Contains(intent, "destroy"),
		strings.Contains(intent, "delete"),
		strings.Contains(intent, "terminate"),
		strings.Contains(intent, "iam"),
		strings.Contains(intent, "policy"),
		strings.Contains(intent, "role"),
		strings.Contains(intent, "user"),
		strings.Contains(intent, "network"),
		strings.Contains(intent, "vpc"),
		strings.Contains(intent, "subnet"),
		strings.Contains(intent, "security group"),
		strings.Contains(intent, "firewall"):
		return "high"
	case strings.Contains(intent, "sync"),
		strings.Contains(intent, "restart"),
		strings.Contains(intent, "rollout"),
		strings.Contains(intent, "migrate"),
		strings.Contains(intent, "upgrade"):
		return "medium"
	case strings.Contains(intent, "deploy"),
		strings.Contains(intent, "scale"),
		strings.Contains(intent, "rollback"):
		return "low"
	default:
		return "read"
	}
}

// highRiskActions maps tool names to actions that are always high risk.
var highRiskActions = map[string][]string{
	"kubectl": {"delete", "exec", "apply", "patch", "replace", "drain", "cordon", "taint"},
	"helm":    {"uninstall", "delete"},
	"aws":     {"delete", "terminate", "remove", "destroy", "put-role-policy", "create-role", "delete-role", "attach-role-policy"},
	"vault":   {"delete", "revoke", "destroy"},
	"argocd":  {"delete"},
}

// mediumRiskActions maps tool names to actions that are at least medium risk.
var mediumRiskActions = map[string][]string{
	"kubectl": {"scale", "rollout"},
	"helm":    {"upgrade", "install", "rollback"},
	"argocd":  {"sync"},
	"aws":     {"update", "modify", "put", "create", "run"},
}

// riskFromSteps calculates risk from actual plan step tools and actions.
// Returns the highest risk found across all steps.
func riskFromSteps(steps []PlanStep) string {
	highest := "read"
	for _, step := range steps {
		tool := strings.ToLower(strings.TrimSpace(step.Tool))
		action := strings.ToLower(strings.TrimSpace(step.Action))
		if tool == "" || action == "" {
			continue
		}
		if matchesAny(action, highRiskActions[tool]) {
			return "high"
		}
		if matchesAny(action, mediumRiskActions[tool]) {
			if riskOrd(highest) < riskOrd("medium") {
				highest = "medium"
			}
		} else if action != "get" && action != "list" && action != "describe" && action != "status" && action != "query" && action != "search" && action != "show" {
			if riskOrd(highest) < riskOrd("low") {
				highest = "low"
			}
		}
	}
	return highest
}

// riskFromStepDrafts is like riskFromSteps but operates on planStepDraft.
func riskFromStepDrafts(steps []planStepDraft) string {
	highest := "read"
	for _, step := range steps {
		tool := strings.ToLower(strings.TrimSpace(step.Tool))
		action := strings.ToLower(strings.TrimSpace(step.Action))
		if tool == "" || action == "" {
			continue
		}
		if matchesAny(action, highRiskActions[tool]) {
			return "high"
		}
		if matchesAny(action, mediumRiskActions[tool]) {
			if riskOrd(highest) < riskOrd("medium") {
				highest = "medium"
			}
		} else if action != "get" && action != "list" && action != "describe" && action != "status" && action != "query" && action != "search" && action != "show" {
			if riskOrd(highest) < riskOrd("low") {
				highest = "low"
			}
		}
	}
	return highest
}

func matchesAny(action string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(action, p) {
			return true
		}
	}
	return false
}

// riskOrd returns a numeric ordering for risk levels for comparison.
func riskOrd(level string) int {
	switch level {
	case "read":
		return 0
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	default:
		return 0
	}
}

// effectiveRisk returns the higher of intent-based and step-based risk.
func effectiveRisk(intentRisk string, steps []PlanStep) string {
	stepRisk := riskFromSteps(steps)
	if riskOrd(stepRisk) > riskOrd(intentRisk) {
		return stepRisk
	}
	return intentRisk
}

// effectiveRiskDrafts returns the higher of intent-based and draft-step-based risk.
func effectiveRiskDrafts(intentRisk string, steps []planStepDraft) string {
	stepRisk := riskFromStepDrafts(steps)
	if riskOrd(stepRisk) > riskOrd(intentRisk) {
		return stepRisk
	}
	return intentRisk
}
