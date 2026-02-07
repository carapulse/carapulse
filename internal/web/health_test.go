package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealthz(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	srv.handleHealthz(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if w.Body.String() != `{"status":"ok"}` {
		t.Fatalf("body: %s", w.Body.String())
	}
}

func TestHandleReadyzOK(t *testing.T) {
	srv := &Server{DB: &fakeDB{}}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.handleReadyz(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleReadyzNilDB(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.handleReadyz(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleReadyzTemporalError(t *testing.T) {
	srv := &Server{
		DB:             &fakeDB{},
		TemporalHealth: func(ctx context.Context) error { return errors.New("down") },
	}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.handleReadyz(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleReadyzTemporalOK(t *testing.T) {
	srv := &Server{
		DB:             &fakeDB{},
		TemporalHealth: func(ctx context.Context) error { return nil },
	}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.handleReadyz(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestGoroutineTracker(t *testing.T) {
	gt := NewGoroutineTracker()
	gt.setAlive("test", true)
	checks := gt.Checks()
	if checks["test"] != "ok" {
		t.Fatalf("expected ok, got: %s", checks["test"])
	}
	gt.setAlive("test", false)
	gt.setErr("test", errors.New("failed"))
	checks = gt.Checks()
	if checks["test"] != "failed" {
		t.Fatalf("expected failed, got: %s", checks["test"])
	}
}

func TestGoroutineTrackerStopped(t *testing.T) {
	gt := NewGoroutineTracker()
	gt.setAlive("test", false)
	checks := gt.Checks()
	if checks["test"] != "stopped" {
		t.Fatalf("expected stopped, got: %s", checks["test"])
	}
}

func TestGoroutineTrackerNil(t *testing.T) {
	var gt *GoroutineTracker
	gt.setAlive("x", true)
	gt.setErr("x", errors.New("e"))
	if gt.Checks() != nil {
		t.Fatalf("expected nil")
	}
}

func TestGoroutineTrackerSetErrNilErr(t *testing.T) {
	gt := NewGoroutineTracker()
	gt.setErr("test", nil) // should be a no-op
	if _, exists := gt.lastErr["test"]; exists {
		t.Fatalf("expected no entry for nil error")
	}
}

func TestHandleReadyzWithGoroutinesOK(t *testing.T) {
	gt := NewGoroutineTracker()
	gt.setAlive("poller", true)
	srv := &Server{DB: &fakeDB{}, Goroutines: gt}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.handleReadyz(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleReadyzWithGoroutinesFailed(t *testing.T) {
	gt := NewGoroutineTracker()
	gt.setAlive("poller", false)
	gt.setErr("poller", errors.New("crashed"))
	srv := &Server{DB: &fakeDB{}, Goroutines: gt}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.handleReadyz(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}
