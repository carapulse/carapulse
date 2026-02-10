package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"carapulse/internal/policy"
)

func enableDevMode(t *testing.T) {
	t.Helper()
	SetAuthConfig(AuthConfig{DevMode: true})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })
}

type memoryDB struct {
	missingExecDB
	memories    map[string][]byte
	listPayload []byte
	createErr   error
	listErr     error
	getErr      error
	updateErr   error
	deleteErr   error
}

func newMemoryDB() *memoryDB {
	return &memoryDB{
		memories:    map[string][]byte{},
		listPayload: []byte("[]"),
	}
}

func (m *memoryDB) CreateOperatorMemory(ctx context.Context, payload []byte) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	m.memories["mem_1"] = payload
	return "mem_1", nil
}

func (m *memoryDB) ListOperatorMemory(ctx context.Context, tenantID string) ([]byte, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.listPayload, nil
}

func (m *memoryDB) GetOperatorMemory(ctx context.Context, memoryID string) ([]byte, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if data, ok := m.memories[memoryID]; ok {
		return data, nil
	}
	return nil, nil
}

func (m *memoryDB) UpdateOperatorMemory(ctx context.Context, memoryID string, payload []byte) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.memories[memoryID] = payload
	return nil
}

func (m *memoryDB) DeleteOperatorMemory(ctx context.Context, memoryID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.memories, memoryID)
	return nil
}

func TestHandleOperatorMemoryCreate(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body, _ := json.Marshal(OperatorMemoryRequest{TenantID: "t", Title: "test", Body: "body"})
	req := httptest.NewRequest(http.MethodPost, "/v1/memory", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemory)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["memory_id"] != "mem_1" {
		t.Fatalf("memory_id: %v", resp["memory_id"])
	}
}

func TestHandleOperatorMemoryCreateMissingFields(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body, _ := json.Marshal(OperatorMemoryRequest{TenantID: "t", Title: "", Body: ""})
	req := httptest.NewRequest(http.MethodPost, "/v1/memory", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemory)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryCreateInvalidJSON(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodPost, "/v1/memory", bytes.NewReader([]byte("{")))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemory)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryCreatePolicyDeny(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: denyChecker{}}}
	body, _ := json.Marshal(OperatorMemoryRequest{TenantID: "t", Title: "test", Body: "body"})
	req := httptest.NewRequest(http.MethodPost, "/v1/memory", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemory)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryList(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}, FailOpenReads: true}
	req := httptest.NewRequest(http.MethodGet, "/v1/memory?tenant_id=t", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemory)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandleOperatorMemoryListMissingTenant(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/memory", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemory)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryStoreUnavailable(t *testing.T) {
	enableDevMode(t)
	srv := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/memory?tenant_id=t", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemory)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryMethodNotAllowed(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodPatch, "/v1/memory", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemory)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryByIDGet(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	db.memories["mem_1"] = []byte(`{"memory_id":"mem_1","tenant_id":"t","title":"test"}`)
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}, FailOpenReads: true}
	req := httptest.NewRequest(http.MethodGet, "/v1/memory/mem_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemoryByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandleOperatorMemoryByIDGetNotFound(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}, FailOpenReads: true}
	req := httptest.NewRequest(http.MethodGet, "/v1/memory/missing", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemoryByID)).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryByIDGetMissingTenant(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/memory/mem_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemoryByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryByIDUpdate(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body, _ := json.Marshal(OperatorMemoryRequest{TenantID: "t", Title: "updated", Body: "new body"})
	req := httptest.NewRequest(http.MethodPut, "/v1/memory/mem_1", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemoryByID)).ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandleOperatorMemoryByIDUpdateMissingFields(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	body, _ := json.Marshal(OperatorMemoryRequest{TenantID: "t", Title: "", Body: ""})
	req := httptest.NewRequest(http.MethodPut, "/v1/memory/mem_1", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemoryByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryByIDDelete(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	db.memories["mem_1"] = []byte(`{}`)
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodDelete, "/v1/memory/mem_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemoryByID)).ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandleOperatorMemoryByIDDeleteMissingTenant(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodDelete, "/v1/memory/mem_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemoryByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryByIDDeletePolicyDeny(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: denyChecker{}}}
	req := httptest.NewRequest(http.MethodDelete, "/v1/memory/mem_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemoryByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryByIDMethodNotAllowed(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodPatch, "/v1/memory/mem_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemoryByID)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryByIDStoreUnavailable(t *testing.T) {
	enableDevMode(t)
	srv := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/memory/mem_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemoryByID)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleOperatorMemoryByIDMissingID(t *testing.T) {
	enableDevMode(t)
	db := newMemoryDB()
	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/memory/", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleOperatorMemoryByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}
