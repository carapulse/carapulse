package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUIHandlers(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/ui/playbooks", nil)
	w := httptest.NewRecorder()
	s.handleUIPlaybooks(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Playbooks") {
		t.Fatalf("body: %s", w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/ui/runbooks", nil)
	w = httptest.NewRecorder()
	s.handleUIRunbooks(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/ui/workflows", nil)
	w = httptest.NewRecorder()
	s.handleUIWorkflows(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/ui/plans/p1", nil)
	w = httptest.NewRecorder()
	s.handleUIPlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Plan p1") {
		t.Fatalf("body: %s", w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/ui/plans/p1", nil)
	w = httptest.NewRecorder()
	s.handleUIPlan(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestUIPageBuilders(t *testing.T) {
	page := uiPage("Title", "/v1/path")
	if !strings.Contains(page, "Title") || !strings.Contains(page, "/v1/path") {
		t.Fatalf("page: %s", page)
	}
	plan := uiPlanPage("plan_1")
	if !strings.Contains(plan, "plan_1") {
		t.Fatalf("plan: %s", plan)
	}
}
