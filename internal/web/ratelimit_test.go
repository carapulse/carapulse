package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(10, 5)
	ip := "192.168.1.1"

	// First 5 requests should be allowed (burst).
	for i := 0; i < 5; i++ {
		if !rl.Allow(ip) {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	// 6th request should be denied (burst exhausted).
	if rl.Allow(ip) {
		t.Fatal("6th request should be denied")
	}
}

func TestRateLimiterTokenRefill(t *testing.T) {
	now := time.Now()
	rateLimitNow = func() time.Time { return now }
	defer func() { rateLimitNow = time.Now }()

	rl := NewRateLimiter(10, 5)
	ip := "10.0.0.1"

	// Exhaust burst.
	for i := 0; i < 5; i++ {
		rl.Allow(ip)
	}
	if rl.Allow(ip) {
		t.Fatal("should be denied after burst")
	}

	// Advance 200ms => 2 tokens refilled (10/s * 0.2s = 2).
	now = now.Add(200 * time.Millisecond)
	if !rl.Allow(ip) {
		t.Fatal("should allow after refill")
	}
	if !rl.Allow(ip) {
		t.Fatal("should allow second after refill")
	}
	if rl.Allow(ip) {
		t.Fatal("should deny after refilled tokens used")
	}
}

func TestRateLimiterSweepStaleEntries(t *testing.T) {
	now := time.Now()
	rateLimitNow = func() time.Time { return now }
	defer func() { rateLimitNow = time.Now }()

	rl := NewRateLimiter(10, 5)
	rl.Allow("stale-ip")

	// Advance 11 minutes to trigger sweep.
	now = now.Add(11 * time.Minute)
	rl.Allow("fresh-ip")

	rl.mu.Lock()
	_, staleExists := rl.clients["stale-ip"]
	_, freshExists := rl.clients["fresh-ip"]
	rl.mu.Unlock()

	if staleExists {
		t.Fatal("stale entry should have been swept")
	}
	if !freshExists {
		t.Fatal("fresh entry should exist")
	}
}

func TestRateLimiterDifferentIPs(t *testing.T) {
	rl := NewRateLimiter(10, 2)

	// IP A uses burst.
	rl.Allow("a")
	rl.Allow("a")
	if rl.Allow("a") {
		t.Fatal("ip a should be denied")
	}

	// IP B should still be allowed.
	if !rl.Allow("b") {
		t.Fatal("ip b should be allowed")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter(10, 1)
	handler := RateLimitMiddleware(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request OK.
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Second request rate limited.
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestClientIPFromXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 172.16.0.1")
	if got := clientIP(req); got != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %s", got)
	}
}

func TestClientIPFromRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:4321"
	if got := clientIP(req); got != "192.168.1.1" {
		t.Fatalf("expected 192.168.1.1, got %s", got)
	}
}

func TestWithRateLimitNil(t *testing.T) {
	srv := &Server{Mux: http.NewServeMux()}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := srv.withRateLimit(inner)
	// Should be the same handler since RateLimiter is nil.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
