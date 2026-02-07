package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"carapulse/internal/policy"
)

func TestHandleRunbooksPost(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body := `{"service":"svc","name":"rb","tenant_id":"t"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandleRunbooksPostMissingService(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body := `{"name":"rb","tenant_id":"t"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbooksPostMissingName(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body := `{"service":"svc","tenant_id":"t"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbooksPostNoTenant(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body := `{"service":"svc","name":"rb"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbooksPostNoDB(t *testing.T) {
	srv := &Server{}
	body := `{"service":"svc","name":"rb","tenant_id":"t"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbooksPostInvalidJSON(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body := `not json`
	req := httptest.NewRequest(http.MethodPost, "/v1/runbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbooksPostPolicyDeny(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: denyChecker{}}}
	body := `{"service":"svc","name":"rb","tenant_id":"t"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbooksGet(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/runbooks", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbooksGetNoTenant(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/runbooks", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbooksGetNoDB(t *testing.T) {
	srv := &Server{Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/runbooks", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbooksMethodNotAllowed(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodDelete, "/v1/runbooks", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbookByIDGet(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/runbooks/runbook_1", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandleRunbookByIDNoTenant(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/runbooks/runbook_1", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbookByIDNoDB(t *testing.T) {
	srv := &Server{Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/runbooks/runbook_1", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbookByIDTenantMismatch(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/runbooks/runbook_1", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "other-tenant")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbookByIDMethodNotAllowed(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/v1/runbooks/runbook_1", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbooksPostTenantFromHeader(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body := `{"service":"svc","name":"rb"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}
