package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"carapulse/internal/policy"
)

func TestHandleWorkflowsGet(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/workflows", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflows)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["data"] == nil {
		t.Fatalf("expected data key")
	}
	if resp["pagination"] == nil {
		t.Fatalf("expected pagination key")
	}
}

func TestHandleWorkflowsGetNoTenant(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/workflows", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflows)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowsMethodNotAllowed(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflows)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowsGetNoDB(t *testing.T) {
	// Without DB, handler falls back to built-in catalog
	srv := &Server{Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/workflows", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflows)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowByIDGetTemplate(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/workflows/gitops_deploy", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["name"] != "gitops_deploy" {
		t.Fatalf("name: %v", resp["name"])
	}
}

func TestHandleWorkflowByIDGetNotFound(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/workflows/nonexistent", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowByIDGetNoTenant(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/workflows/gitops_deploy", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowByIDStartPost(t *testing.T) {
	srv := &Server{
		DB:       &fakeDB{},
		Policy:   &policy.Evaluator{Checker: allowChecker{}},
		Executor: &fakeExecutor{},
	}
	ctx := validContext()
	body, _ := json.Marshal(WorkflowStartRequest{
		Context: ctx,
		Input:   map[string]any{"argocd_app": "myapp"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/gitops_deploy/start", bytes.NewReader(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
	var resp WorkflowStartResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.PlanID == "" {
		t.Fatalf("expected plan_id")
	}
}

func TestHandleWorkflowByIDStartNoExecutor(t *testing.T) {
	srv := &Server{
		DB:     &fakeDB{},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
		// Executor intentionally nil — Temporal not configured
	}
	ctx := validContext()
	body, _ := json.Marshal(WorkflowStartRequest{
		Context: ctx,
		Input:   map[string]any{"argocd_app": "myapp"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/gitops_deploy/start", bytes.NewReader(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when no executor, got: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandleWorkflowByIDStartInvalidContext(t *testing.T) {
	srv := &Server{
		DB:     &fakeDB{},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	body, _ := json.Marshal(WorkflowStartRequest{
		Context: ContextRef{TenantID: "t"}, // incomplete context
		Input:   map[string]any{"argocd_app": "myapp"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/gitops_deploy/start", bytes.NewReader(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowByIDStartMissingTemplate(t *testing.T) {
	srv := &Server{
		DB:     &fakeDB{},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	ctx := validContext()
	body, _ := json.Marshal(WorkflowStartRequest{
		Context: ctx,
		Input:   map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/nonexistent/start", bytes.NewReader(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowByIDStartNoDB(t *testing.T) {
	srv := &Server{
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	ctx := validContext()
	body, _ := json.Marshal(WorkflowStartRequest{
		Context: ctx,
		Input:   map[string]any{"argocd_app": "myapp"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/gitops_deploy/start", bytes.NewReader(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowByIDStartInvalidJSON(t *testing.T) {
	srv := &Server{
		DB:     &fakeDB{},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/gitops_deploy/start", bytes.NewBufferString("not json"))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowByIDStartMissingInput(t *testing.T) {
	srv := &Server{
		DB:     &fakeDB{},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	ctx := validContext()
	// gitops_deploy requires argocd_app, so empty input should fail
	body, _ := json.Marshal(WorkflowStartRequest{
		Context: ctx,
		Input:   map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/gitops_deploy/start", bytes.NewReader(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowByIDStartPolicyDeny(t *testing.T) {
	srv := &Server{
		DB:     &fakeDB{},
		Policy: &policy.Evaluator{Checker: denyChecker{}},
	}
	ctx := validContext()
	body, _ := json.Marshal(WorkflowStartRequest{
		Context: ctx,
		Input:   map[string]any{"argocd_app": "myapp"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/gitops_deploy/start", bytes.NewReader(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowByIDVersionPost(t *testing.T) {
	srv := &Server{
		DB:     &fakeDB{},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	body := `{"tenant_id":"t","version":2,"description":"updated"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/gitops_deploy/version", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandleWorkflowByIDVersionPostNoTenant(t *testing.T) {
	srv := &Server{
		DB:     &fakeDB{},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	body := `{"version":2,"description":"updated"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/gitops_deploy/version", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestFindWorkflowByName(t *testing.T) {
	payload := `[{"name":"wf1","version":1,"tenant_id":"t"},{"name":"wf1","version":2,"tenant_id":"t"},{"name":"wf2","version":1,"tenant_id":"t"}]`
	found, ok := findWorkflowByName([]byte(payload), "wf1")
	if !ok {
		t.Fatalf("expected to find wf1")
	}
	if v, _ := found["version"].(float64); int(v) != 2 {
		t.Fatalf("expected version 2, got: %v", found["version"])
	}
}

func TestFindWorkflowByNameNotFound(t *testing.T) {
	payload := `[{"name":"wf1","version":1}]`
	_, ok := findWorkflowByName([]byte(payload), "missing")
	if ok {
		t.Fatalf("expected not found")
	}
}

func TestFindWorkflowByNameEmptyName(t *testing.T) {
	payload := `[{"name":"wf1","version":1}]`
	_, ok := findWorkflowByName([]byte(payload), "")
	if ok {
		t.Fatalf("expected not found for empty name")
	}
}

func TestFindWorkflowByNameBadJSON(t *testing.T) {
	_, ok := findWorkflowByName([]byte("not json"), "wf1")
	if ok {
		t.Fatalf("expected not found for bad json")
	}
}

func TestFindWorkflowByNameIntVersion(t *testing.T) {
	// JSON numbers are float64 by default but we test the code handles both
	payload := `[{"name":"wf1","version":3}]`
	found, ok := findWorkflowByName([]byte(payload), "wf1")
	if !ok {
		t.Fatalf("expected to find wf1")
	}
	if found["name"] != "wf1" {
		t.Fatalf("name: %v", found["name"])
	}
}

func TestHandleWorkflowByIDEmptyName(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/workflows/", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowByIDMethodNotAllowed(t *testing.T) {
	srv := &Server{DB: &fakeDB{}}
	req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/gitops_deploy/unknown", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}
