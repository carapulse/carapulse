package tools

import "strings"

func actionTypeForTool(tool, action string) string {
	tool = strings.ToLower(strings.TrimSpace(tool))
	action = strings.ToLower(strings.TrimSpace(action))
	switch tool {
	case "prometheus", "thanos", "tempo":
		return "read"
	case "grafana":
		if action == "dashboard_get" {
			return "read"
		}
		return "write"
	case "argocd":
		if action == "wait" || action == "status" || action == "list" {
			return "read"
		}
		return "write"
	case "kubectl":
		if action == "rollout-status" || action == "get" {
			return "read"
		}
		return "write"
	case "helm":
		if action == "status" || action == "list" || action == "get" {
			return "read"
		}
		return "write"
	case "aws":
		if strings.Contains(action, "lookup") || strings.Contains(action, "get") || strings.Contains(action, "list") {
			return "read"
		}
		return "write"
	case "vault":
		if strings.Contains(action, "read") {
			return "read"
		}
		return "write"
	case "boundary":
		if strings.Contains(action, "list") || strings.Contains(action, "get") {
			return "read"
		}
		return "write"
	default:
		return "write"
	}
}

func riskForToolAction(tool, action string) string {
	action = strings.ToLower(strings.TrimSpace(action))
	switch {
	case strings.Contains(action, "delete"),
		strings.Contains(action, "terminate"),
		strings.Contains(action, "rollback"),
		strings.Contains(action, "iam"),
		strings.Contains(action, "policy"),
		strings.Contains(action, "network"):
		return "high"
	case strings.Contains(action, "sync"),
		strings.Contains(action, "restart"),
		strings.Contains(action, "upgrade"),
		strings.Contains(action, "scale"):
		return "medium"
	default:
		if actionTypeForTool(tool, action) == "read" {
			return "read"
		}
		return "low"
	}
}

func tierForRisk(risk string) string {
	switch strings.ToLower(strings.TrimSpace(risk)) {
	case "read":
		return "read"
	case "low", "medium":
		return "safe"
	default:
		return "break_glass"
	}
}

func blastRadiusForContext(ctx ContextRef) string {
	if strings.TrimSpace(ctx.Namespace) != "" {
		return "namespace"
	}
	if strings.TrimSpace(ctx.ClusterID) != "" {
		return "cluster"
	}
	return "account"
}
