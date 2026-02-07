package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestJWKSCacheGetCaches(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[{"kid":"1","kty":"RSA","n":"AA","e":"AQAB"}]}`))
	}))
	defer srv.Close()

	c := NewJWKSCache(5 * time.Minute)
	t.Cleanup(func() { c.Close() })
	j1, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(j1.Keys) != 1 || j1.Keys[0].Kid != "1" {
		t.Fatalf("jwks: %#v", j1)
	}

	j2, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(j2.Keys) != 1 || j2.Keys[0].Kid != "1" {
		t.Fatalf("jwks: %#v", j2)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls: %d", calls.Load())
	}
}

func TestJWKSCacheNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()

	c := NewJWKSCache(time.Minute)
	t.Cleanup(func() { c.Close() })
	if _, err := c.Get(context.Background(), srv.URL); err == nil {
		t.Fatalf("expected error")
	}
}

func TestJWKSCacheBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()

	c := NewJWKSCache(time.Minute)
	t.Cleanup(func() { c.Close() })
	if _, err := c.Get(context.Background(), srv.URL); err == nil {
		t.Fatalf("expected error")
	}
}

func TestJWKSCacheInvalidURL(t *testing.T) {
	c := NewJWKSCache(time.Minute)
	t.Cleanup(func() { c.Close() })
	if _, err := c.Get(context.Background(), "http://[::1"); err == nil {
		t.Fatalf("expected error")
	}
}
