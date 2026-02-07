package web

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type TemporalHealthFunc func(context.Context) error

type GoroutineTracker struct {
	mu     sync.Mutex
	alive  map[string]bool
	lastErr map[string]string
}

func NewGoroutineTracker() *GoroutineTracker {
	return &GoroutineTracker{
		alive:   map[string]bool{},
		lastErr: map[string]string{},
	}
}

func (t *GoroutineTracker) setAlive(name string, alive bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.alive[name] = alive
}

func (t *GoroutineTracker) setErr(name string, err error) {
	if t == nil || err == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastErr[name] = err.Error()
}

func (t *GoroutineTracker) Checks() map[string]string {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	out := map[string]string{}
	for name, alive := range t.alive {
		if alive {
			out[name] = "ok"
			continue
		}
		if msg := t.lastErr[name]; msg != "" {
			out[name] = msg
		} else {
			out[name] = "stopped"
		}
	}
	return out
}

func (t *GoroutineTracker) Go(ctx context.Context, wg *sync.WaitGroup, name string, fn func(context.Context) error) {
	if t == nil {
		if wg != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = fn(ctx)
			}()
		} else {
			go func() { _ = fn(ctx) }()
		}
		return
	}
	if wg != nil {
		wg.Add(1)
	}
	t.setAlive(name, true)
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		defer t.setAlive(name, false)
		if err := fn(ctx); err != nil && ctx.Err() == nil {
			t.setErr(name, err)
		}
	}()
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	checks := map[string]string{}
	ok := true

	if s == nil || s.DB == nil {
		ok = false
		checks["db"] = "unavailable"
	} else if s.DBConn != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.DBConn.PingContext(ctx); err != nil {
			ok = false
			checks["db"] = err.Error()
		} else {
			checks["db"] = "ok"
		}
	} else {
		checks["db"] = "unknown"
	}

	if s != nil && s.TemporalHealth != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.TemporalHealth(ctx); err != nil {
			ok = false
			checks["temporal"] = err.Error()
		} else {
			checks["temporal"] = "ok"
		}
	}

	if s != nil && s.Goroutines != nil {
		for name, status := range s.Goroutines.Checks() {
			if status != "ok" {
				ok = false
			}
			checks["goroutine."+name] = status
		}
	}

	if ok {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	if data, err := marshalJSON(map[string]any{"status": "unavailable", "checks": checks}); err == nil {
		_, _ = w.Write(data)
		return
	}
	_, _ = w.Write([]byte(`{"status":"unavailable"}`))
}
