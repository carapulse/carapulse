package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"carapulse/internal/policy"
)

func TestHandlePlaybooksPost(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body := `{"name":"pb","tenant_id":"t","spec":{"key":"val"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/playbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandlePlaybooksPostNoName(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body := `{"name":"","tenant_id":"t","spec":{"key":"val"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/playbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybooksPostNoSpec(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body := `{"name":"pb","tenant_id":"t"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/playbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybooksPostNoTenant(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body := `{"name":"pb","spec":{"key":"val"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/playbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybooksPostNoDB(t *testing.T) {
	srv := &Server{}
	body := `{"name":"pb","tenant_id":"t","spec":{"key":"val"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/playbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybooksPostInvalidJSON(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body := `not json`
	req := httptest.NewRequest(http.MethodPost, "/v1/playbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybooksPostPolicyDeny(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: denyChecker{}}}
	body := `{"name":"pb","tenant_id":"t","spec":{"key":"val"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/playbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybooksGet(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/playbooks", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybooksGetNoTenant(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/playbooks", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybooksGetNoDB(t *testing.T) {
	srv := &Server{Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/playbooks", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybooksMethodNotAllowed(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodDelete, "/v1/playbooks", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybookByIDGet(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/playbooks/playbook_1", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandlePlaybookByIDNoTenant(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/playbooks/playbook_1", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybookByIDNoDB(t *testing.T) {
	srv := &Server{Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/playbooks/playbook_1", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybookByIDMethodNotAllowed(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/v1/playbooks/playbook_1", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybooksPostTenantFromHeader(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body := `{"name":"pb","spec":{"key":"val"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/playbooks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandlePlaybookByIDDelete(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodDelete, "/v1/playbooks/playbook_1", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandlePlaybookByIDDeleteNoDB(t *testing.T) {
	srv := &Server{Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodDelete, "/v1/playbooks/playbook_1", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybookByIDDeleteDBError(t *testing.T) {
	srv := &Server{DB: errorDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodDelete, "/v1/playbooks/playbook_1", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handlePlaybookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}
