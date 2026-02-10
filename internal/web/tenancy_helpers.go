package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func resolveTenant(bodyTenant, headerTenant string) (string, error) {
	bodyTenant = strings.TrimSpace(bodyTenant)
	headerTenant = strings.TrimSpace(headerTenant)
	if bodyTenant == "" {
		bodyTenant = headerTenant
	} else if headerTenant != "" && headerTenant != bodyTenant {
		return "", errors.New("tenant_id mismatch")
	}
	if bodyTenant == "" {
		return "", errors.New("tenant_id required")
	}
	return bodyTenant, nil
}

func tenantFromMap(item map[string]any) string {
	if item == nil {
		return ""
	}
	if v, ok := item["tenant_id"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func tenantMatch(itemTenant, tenantID string, includeGlobal bool) bool {
	itemTenant = strings.TrimSpace(itemTenant)
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return false
	}
	if itemTenant == "" {
		return includeGlobal
	}
	return itemTenant == tenantID
}

func filterItemsByTenant(items []map[string]any, tenantID string, includeGlobal bool) []map[string]any {
	if len(items) == 0 {
		return items
	}
	filtered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if tenantMatch(tenantFromMap(item), tenantID, includeGlobal) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterJSONByTenant(payload []byte, tenantID string, includeGlobal bool) ([]byte, error) {
	var items []map[string]any
	if err := json.Unmarshal(payload, &items); err != nil {
		return nil, err
	}
	filtered := filterItemsByTenant(items, tenantID, includeGlobal)
	return json.Marshal(filtered)
}

func policyCheckTenantRead(s *Server, r *http.Request, action, tenantID string) error {
	if s != nil && s.SessionRequired {
		if _, err := s.requireSession(r); err != nil {
			return err
		}
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return errors.New("tenant_id required")
	}
	return s.policyCheck(r, action, "read", ContextRef{TenantID: tenantID}, "read", 0)
}

func tenantFromContext(item map[string]any) string {
	if item == nil {
		return ""
	}
	ctx, ok := item["context"].(map[string]any)
	if !ok || ctx == nil {
		return ""
	}
	if v, ok := ctx["tenant_id"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func filterJSONByTenantContext(payload []byte, tenantID string) ([]byte, error) {
	var items []map[string]any
	if err := json.Unmarshal(payload, &items); err != nil {
		return nil, err
	}
	filtered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if tenantMatch(tenantFromContext(item), tenantID, false) {
			filtered = append(filtered, item)
		}
	}
	return json.Marshal(filtered)
}

func tenantFromLabels(item map[string]any) string {
	if item == nil {
		return ""
	}
	labels, ok := item["labels"].(map[string]any)
	if !ok || labels == nil {
		return ""
	}
	if v, ok := labels["tenant_id"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func filterItemsByLabelTenant(items []map[string]any, tenantID string, includeGlobal bool) []map[string]any {
	if len(items) == 0 {
		return items
	}
	filtered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if tenantMatch(tenantFromLabels(item), tenantID, includeGlobal) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
