package web

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type rateLimitEntry struct {
	tokens    float64
	lastCheck time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*rateLimitEntry
	rate     float64 // tokens per second
	burst    int     // max tokens
	lastSweep time.Time
}

var rateLimitNow = time.Now

func NewRateLimiter(ratePerSecond float64, burst int) *RateLimiter {
	return &RateLimiter{
		clients:   make(map[string]*rateLimitEntry),
		rate:      ratePerSecond,
		burst:     burst,
		lastSweep: rateLimitNow(),
	}
}

func (rl *RateLimiter) Allow(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rateLimitNow()

	// Sweep stale entries every 5 minutes.
	if now.Sub(rl.lastSweep) > 5*time.Minute {
		cutoff := now.Add(-10 * time.Minute)
		for k, v := range rl.clients {
			if v.lastCheck.Before(cutoff) {
				delete(rl.clients, k)
			}
		}
		rl.lastSweep = now
	}

	entry, ok := rl.clients[clientIP]
	if !ok {
		entry = &rateLimitEntry{
			tokens:    float64(rl.burst) - 1,
			lastCheck: now,
		}
		rl.clients[clientIP] = entry
		return true
	}

	elapsed := now.Sub(entry.lastCheck).Seconds()
	entry.tokens += elapsed * rl.rate
	if entry.tokens > float64(rl.burst) {
		entry.tokens = float64(rl.burst)
	}
	entry.lastCheck = now

	if entry.tokens < 1 {
		return false
	}
	entry.tokens--
	return true
}

func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !limiter.Allow(ip) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (client IP) from the chain.
		for j := 0; j < len(xff); j++ {
			if xff[j] == ',' {
				return xff[:j]
			}
		}
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
