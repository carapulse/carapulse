package web

import "strings"

func normalizeApprovalStatus(status string) (string, bool) {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		return "pending", true
	}
	switch status {
	case "pending", "approved", "denied", "expired":
		return status, true
	default:
		return "", false
	}
}
