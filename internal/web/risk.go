package web

import "strings"

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

func blastRadius(ctx ContextRef, targets int) string {
	if targets <= 0 {
		return "namespace"
	}
	if targets <= 10 && strings.TrimSpace(ctx.Namespace) != "" {
		return "namespace"
	}
	if targets <= 50 {
		return "cluster"
	}
	return "account"
}
