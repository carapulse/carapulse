package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"carapulse/internal/policy"
)

type snapshotListDB struct {
	missingExecDB
	listPayload []byte
	listErr     error
	getPayload  []byte
	getErr      error
}

func (s snapshotListDB) ListContextSnapshots(ctx context.Context, limit, offset int) ([]byte, int, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	return s.listPayload, 2, nil
}

func (s snapshotListDB) GetContextSnapshot(ctx context.Context, snapshotID string) ([]byte, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.getPayload != nil {
		return s.getPayload, nil
	}
	return nil, nil
}

func TestHandleContextSnapshotsGetOK(t *testing.T) {
	payload := `[{"snapshot_id":"s1","labels":{"tenant_id":"t"}},{"snapshot_id":"s2","labels":{"tenant_id":"other"}}]`
	srv := &Server{
		DB:     snapshotListDB{listPayload: []byte(payload)},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshots)).ServeHTTP(w, req)
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
	items, ok := resp["data"].([]any)
	if !ok {
		t.Fatalf("data not array")
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item after tenant filter, got %d", len(items))
	}
}

func TestHandleContextSnapshotsGetNoTenant(t *testing.T) {
	srv := &Server{
		DB:     &fakeDB{},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshots)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotsGetPolicyDeny(t *testing.T) {
	srv := &Server{
		DB:     &fakeDB{},
		Policy: &policy.Evaluator{Checker: denyChecker{}},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshots)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotsGetNoDB(t *testing.T) {
	srv := &Server{
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshots)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotsMethodNotAllowed(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/v1/context/snapshots", nil)
	req.Header.Set("Authorization", testToken)
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshots)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotsGetDBError(t *testing.T) {
	srv := &Server{
		DB:     snapshotListDB{listErr: errTest},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshots)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotByIDGetDBError(t *testing.T) {
	srv := &Server{
		DB:     snapshotListDB{getErr: errTest},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/snap_1", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandleContextSnapshotByIDGetNilPayload(t *testing.T) {
	srv := &Server{
		DB:     snapshotListDB{},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/snap_1", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotByIDGetTenantMismatch(t *testing.T) {
	payload := `{"snapshot_id":"snap_1","labels":{"tenant_id":"other"},"nodes":[],"edges":[]}`
	srv := &Server{
		DB:     snapshotListDB{getPayload: []byte(payload)},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/snap_1", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotByIDDiffDBError(t *testing.T) {
	srv := &Server{
		DB:     snapshotListDB{getErr: errTest},
		Policy: &policy.Evaluator{Checker: allowChecker{}},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/snap_1/diff?base=snap_2", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotByIDDiffPolicyDeny(t *testing.T) {
	srv := &Server{
		DB:     snapshotListDB{},
		Policy: &policy.Evaluator{Checker: denyChecker{}},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/snap_1/diff?base=snap_2", nil)
	req.Header.Set("Authorization", testToken)
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}
