package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"carapulse/internal/policy"
)

// TestMaxBytesReaderPlansPost verifies oversized bodies are rejected on POST /v1/plans.
func TestMaxBytesReaderPlansPost(t *testing.T) {
	body := strings.Repeat("x", maxRequestBody+1)
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", w.Code)
	}
}

// TestMaxBytesReaderApprovalsPost verifies oversized bodies are rejected on POST /v1/approvals.
func TestMaxBytesReaderApprovalsPost(t *testing.T) {
	body := strings.Repeat("x", maxRequestBody+1)
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", w.Code)
	}
}

// TestMaxBytesReaderSchedulesPost verifies oversized bodies are rejected on POST /v1/schedules.
func TestMaxBytesReaderSchedulesPost(t *testing.T) {
	body := strings.Repeat("x", maxRequestBody+1)
	req := httptest.NewRequest(http.MethodPost, "/v1/schedules", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handleSchedules)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", w.Code)
	}
}

// TestMaxBytesReaderContextRefresh verifies oversized bodies are rejected on POST /v1/context/refresh.
func TestMaxBytesReaderContextRefresh(t *testing.T) {
	body := strings.Repeat("x", maxRequestBody+1)
	req := httptest.NewRequest(http.MethodPost, "/v1/context/refresh", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}, Context: &fakeContextManager{}}
	AuthMiddleware(http.HandlerFunc(srv.handleContextRefresh)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", w.Code)
	}
}

// TestMaxBytesReaderHookPost verifies oversized bodies are rejected on POST /v1/hooks/*.
func TestMaxBytesReaderHookPost(t *testing.T) {
	body := strings.Repeat("x", maxRequestBody+1)
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/alertmanager", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", w.Code)
	}
}

// activeExecDB implements ActiveExecutionChecker for idempotency tests.
type activeExecDB struct {
	fakeDB
	hasActive bool
	checkErr  error
}

func (a *activeExecDB) HasActiveExecution(ctx context.Context, planID string) (bool, error) {
	return a.hasActive, a.checkErr
}

// TestExecutionIdempotencyBlocked verifies that executing a plan with an active execution returns 409.
func TestExecutionIdempotencyBlocked(t *testing.T) {
	db := &activeExecDB{hasActive: true}
	ctx := validContext()
	plan := map[string]any{
		"intent":     "deploy",
		"context":    ctx,
		"risk_level": "read",
	}
	planJSON := mustPlanJSON(t, plan)
	db.lastPlan = planJSON
	db.planID = "plan_1"
	db.approvalStatus = "approved"

	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestExecutionIdempotencyAllowed verifies execution proceeds when no active execution exists.
func TestExecutionIdempotencyAllowed(t *testing.T) {
	db := &activeExecDB{hasActive: false}
	ctx := validContext()
	plan := map[string]any{
		"intent":     "deploy",
		"context":    ctx,
		"risk_level": "read",
	}
	planJSON := mustPlanJSON(t, plan)
	db.lastPlan = planJSON
	db.planID = "plan_1"
	db.approvalStatus = "approved"

	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["execution_id"]; !ok {
		t.Fatal("expected execution_id in response")
	}
}

// TestExecutionIdempotencyCheckError verifies DB error in idempotency check returns 500.
func TestExecutionIdempotencyCheckError(t *testing.T) {
	db := &activeExecDB{checkErr: errTest}
	ctx := validContext()
	plan := map[string]any{
		"intent":     "deploy",
		"context":    ctx,
		"risk_level": "read",
	}
	planJSON := mustPlanJSON(t, plan)
	db.lastPlan = planJSON
	db.planID = "plan_1"
	db.approvalStatus = "approved"

	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestMaxRequestBodyConstant verifies the constant is exactly 1MB.
func TestMaxRequestBodyConstant(t *testing.T) {
	if maxRequestBody != 1<<20 {
		t.Fatalf("maxRequestBody should be 1<<20 (1MB), got %d", maxRequestBody)
	}
}
