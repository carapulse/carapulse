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
