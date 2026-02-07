package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"carapulse/internal/policy"
)

func TestHandleSchedulesPostAndGet(t *testing.T) {
	db := &fakeDB{}
	s := &Server{
		DB: db,
		Policy: &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) {
			return policy.PolicyDecision{Decision: "allow"}, nil
		})},
	}
	ctx := ContextRef{
		TenantID:      "t",
		Environment:   "dev",
		ClusterID:     "c",
		Namespace:     "ns",
		AWSAccountID:  "a",
		Region:        "r",
		ArgoCDProject: "p",
		GrafanaOrgID:  "g",
	}
	body, _ := json.Marshal(ScheduleCreateRequest{Summary: "sum", Intent: "scale service", Cron: "* * * * *", Context: ctx, Enabled: true})
	req := httptest.NewRequest(http.MethodPost, "/v1/schedules", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleSchedules(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/schedules", nil)
	w = httptest.NewRecorder()
	s.handleSchedules(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleSchedulesErrors(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/v1/schedules", nil)
	w := httptest.NewRecorder()
	s.handleSchedules(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
	db := &fakeDB{}
	s = &Server{
		DB: db,
		Policy: &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) {
			return policy.PolicyDecision{Decision: "deny"}, nil
		})},
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/schedules", bytes.NewReader([]byte("{}")))
	w = httptest.NewRecorder()
	s.handleSchedules(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/schedules", bytes.NewReader([]byte(`{"summary":"s","cron":"* * * * *","context":{}}`)))
	w = httptest.NewRecorder()
	s.handleSchedules(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/schedules", bytes.NewReader([]byte(`{"summary":"s","cron":"* * * * *","context":{"tenant_id":"t","environment":"dev","cluster_id":"c","namespace":"ns","aws_account_id":"a","region":"r","argocd_project":"p","grafana_org_id":"g"}}`)))
	w = httptest.NewRecorder()
	s.handleSchedules(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestShouldRunEventLoop(t *testing.T) {
	s := &Server{EnableEventLoop: true}
	if !s.shouldRunEventLoop("alertmanager") {
		t.Fatalf("expected default allow")
	}
	if !s.shouldRunEventLoop("k8s") {
		t.Fatalf("expected default allow for k8s")
	}
	if s.shouldRunEventLoop("unknown") {
		t.Fatalf("unexpected allow")
	}
	s.EventLoopSources = []string{"git"}
	if !s.shouldRunEventLoop("git") {
		t.Fatalf("expected allow")
	}
	if s.shouldRunEventLoop("alertmanager") {
		t.Fatalf("unexpected allow")
	}
}
