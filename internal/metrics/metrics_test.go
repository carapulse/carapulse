package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddleware(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/plans", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
}

func TestHandler(t *testing.T) {
	h := Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "carapulse_http_requests_total") {
		t.Errorf("expected carapulse_http_requests_total in output")
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "/"},
		{"/", "/"},
		{"/healthz", "/healthz"},
		{"/readyz", "/readyz"},
		{"/metrics", "/metrics"},
		{"/v1/plans", "/v1/plans"},
		{"/v1/plans/abc123", "/v1/plans"},
		{"/v1/executions/xyz/logs", "/v1/executions"},
	}
	for _, tt := range tests {
		if got := normalizePath(tt.input); got != tt.want {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResponseWriterStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}
	rw.WriteHeader(http.StatusNotFound)
	if rw.statusCode != http.StatusNotFound {
		t.Errorf("got %d, want %d", rw.statusCode, http.StatusNotFound)
	}
}
