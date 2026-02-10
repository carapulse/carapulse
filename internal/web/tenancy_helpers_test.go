package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"carapulse/internal/policy"
)

func TestResolveTenantBodyOnly(t *testing.T) {
	id, err := resolveTenant("t1", "")
	if err != nil || id != "t1" {
		t.Fatalf("id=%s err=%v", id, err)
	}
}

func TestResolveTenantHeaderOnly(t *testing.T) {
	id, err := resolveTenant("", "t2")
	if err != nil || id != "t2" {
		t.Fatalf("id=%s err=%v", id, err)
	}
}

func TestResolveTenantBothMatch(t *testing.T) {
	id, err := resolveTenant("t1", "t1")
	if err != nil || id != "t1" {
		t.Fatalf("id=%s err=%v", id, err)
	}
}

func TestResolveTenantMismatch(t *testing.T) {
	_, err := resolveTenant("t1", "t2")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveTenantBothEmpty(t *testing.T) {
	_, err := resolveTenant("", "")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestTenantFromMapNil(t *testing.T) {
	if v := tenantFromMap(nil); v != "" {
		t.Fatalf("expected empty, got %s", v)
	}
}

func TestTenantFromMapMissing(t *testing.T) {
	if v := tenantFromMap(map[string]any{"name": "test"}); v != "" {
		t.Fatalf("expected empty, got %s", v)
	}
}

func TestTenantFromMapPresent(t *testing.T) {
	if v := tenantFromMap(map[string]any{"tenant_id": "t1"}); v != "t1" {
		t.Fatalf("expected t1, got %s", v)
	}
}

func TestTenantMatchEmptyTenantID(t *testing.T) {
	if tenantMatch("item_t", "", false) {
		t.Fatalf("expected false for empty tenantID")
	}
}

func TestTenantMatchEmptyItemTenantNoGlobal(t *testing.T) {
	if tenantMatch("", "t1", false) {
		t.Fatalf("expected false for empty item tenant without includeGlobal")
	}
}

func TestTenantMatchEmptyItemTenantWithGlobal(t *testing.T) {
	if !tenantMatch("", "t1", true) {
		t.Fatalf("expected true for empty item tenant with includeGlobal")
	}
}

func TestTenantMatchExact(t *testing.T) {
	if !tenantMatch("t1", "t1", false) {
		t.Fatalf("expected true for matching tenants")
	}
}

func TestTenantMatchDifferent(t *testing.T) {
	if tenantMatch("t1", "t2", false) {
		t.Fatalf("expected false for different tenants")
	}
}

func TestFilterItemsByTenantEmpty(t *testing.T) {
	result := filterItemsByTenant(nil, "t1", false)
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestFilterItemsByTenantMatching(t *testing.T) {
	items := []map[string]any{
		{"tenant_id": "t1", "name": "a"},
		{"tenant_id": "t2", "name": "b"},
		{"tenant_id": "t1", "name": "c"},
	}
	result := filterItemsByTenant(items, "t1", false)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestFilterItemsByTenantNoMatch(t *testing.T) {
	items := []map[string]any{
		{"tenant_id": "t2", "name": "a"},
	}
	result := filterItemsByTenant(items, "t1", false)
	if len(result) != 0 {
		t.Fatalf("expected 0, got %d", len(result))
	}
}

func TestFilterItemsByTenantIncludeGlobal(t *testing.T) {
	items := []map[string]any{
		{"tenant_id": "t1", "name": "a"},
		{"name": "global"},
	}
	result := filterItemsByTenant(items, "t1", true)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestFilterJSONByTenantOK(t *testing.T) {
	payload := []byte(`[{"tenant_id":"t1"},{"tenant_id":"t2"}]`)
	out, err := filterJSONByTenant(payload, "t1", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != `[{"tenant_id":"t1"}]` {
		t.Fatalf("output: %s", out)
	}
}

func TestFilterJSONByTenantInvalidJSON(t *testing.T) {
	if _, err := filterJSONByTenant([]byte("{"), "t1", false); err == nil {
		t.Fatalf("expected error")
	}
}

func TestTenantFromContextNil(t *testing.T) {
	if v := tenantFromContext(nil); v != "" {
		t.Fatalf("expected empty, got %s", v)
	}
}

func TestTenantFromContextMissing(t *testing.T) {
	if v := tenantFromContext(map[string]any{"name": "test"}); v != "" {
		t.Fatalf("expected empty, got %s", v)
	}
}

func TestTenantFromContextPresent(t *testing.T) {
	item := map[string]any{"context": map[string]any{"tenant_id": "t1"}}
	if v := tenantFromContext(item); v != "t1" {
		t.Fatalf("expected t1, got %s", v)
	}
}

func TestTenantFromContextNilContext(t *testing.T) {
	item := map[string]any{"context": nil}
	if v := tenantFromContext(item); v != "" {
		t.Fatalf("expected empty, got %s", v)
	}
}

func TestFilterJSONByTenantContextOK(t *testing.T) {
	payload := []byte(`[{"context":{"tenant_id":"t1"}},{"context":{"tenant_id":"t2"}}]`)
	out, err := filterJSONByTenantContext(payload, "t1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != `[{"context":{"tenant_id":"t1"}}]` {
		t.Fatalf("output: %s", out)
	}
}

func TestFilterJSONByTenantContextInvalidJSON(t *testing.T) {
	if _, err := filterJSONByTenantContext([]byte("{"), "t1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestTenantFromLabelsNil(t *testing.T) {
	if v := tenantFromLabels(nil); v != "" {
		t.Fatalf("expected empty, got %s", v)
	}
}

func TestTenantFromLabelsMissing(t *testing.T) {
	if v := tenantFromLabels(map[string]any{"name": "test"}); v != "" {
		t.Fatalf("expected empty, got %s", v)
	}
}

func TestTenantFromLabelsPresent(t *testing.T) {
	item := map[string]any{"labels": map[string]any{"tenant_id": "t1"}}
	if v := tenantFromLabels(item); v != "t1" {
		t.Fatalf("expected t1, got %s", v)
	}
}

func TestTenantFromLabelsNilLabels(t *testing.T) {
	item := map[string]any{"labels": nil}
	if v := tenantFromLabels(item); v != "" {
		t.Fatalf("expected empty, got %s", v)
	}
}

func TestFilterItemsByLabelTenantEmpty(t *testing.T) {
	result := filterItemsByLabelTenant(nil, "t1", false)
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestFilterItemsByLabelTenantMatching(t *testing.T) {
	items := []map[string]any{
		{"labels": map[string]any{"tenant_id": "t1"}, "name": "a"},
		{"labels": map[string]any{"tenant_id": "t2"}, "name": "b"},
		{"labels": map[string]any{"tenant_id": "t1"}, "name": "c"},
	}
	result := filterItemsByLabelTenant(items, "t1", false)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestFilterItemsByLabelTenantNoMatch(t *testing.T) {
	items := []map[string]any{
		{"labels": map[string]any{"tenant_id": "t2"}, "name": "a"},
	}
	result := filterItemsByLabelTenant(items, "t1", false)
	if len(result) != 0 {
		t.Fatalf("expected 0, got %d", len(result))
	}
}

func TestFilterItemsByLabelTenantIncludeGlobal(t *testing.T) {
	items := []map[string]any{
		{"labels": map[string]any{"tenant_id": "t1"}, "name": "a"},
		{"name": "global"},
	}
	result := filterItemsByLabelTenant(items, "t1", true)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestPolicyCheckTenantReadEmptyTenant(t *testing.T) {
	srv := &Server{Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := policyCheckTenantRead(srv, req, "test", ""); err == nil {
		t.Fatalf("expected error for empty tenant")
	}
}

func TestPolicyCheckTenantReadOK(t *testing.T) {
	srv := &Server{Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", testToken)
	if err := policyCheckTenantRead(srv, req, "test", "t1"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestPolicyCheckTenantReadDeny(t *testing.T) {
	srv := &Server{Policy: &policy.Evaluator{Checker: denyChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", testToken)
	if err := policyCheckTenantRead(srv, req, "test", "t1"); err == nil {
		t.Fatalf("expected error")
	}
}
