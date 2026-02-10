package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"carapulse/internal/auth"
)

func tokenForClaims(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return "aaa." + encoded + ".bbb"
}

func TestAuthMiddlewareRejectsMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rw.Code)
	}
}

func TestAuthMiddlewareAcceptsBearer(t *testing.T) {
	SetAuthConfig(AuthConfig{DevMode: true})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	token := tokenForClaims(t, map[string]any{
		"sub":    "user1",
		"email":  "user@example.com",
		"groups": []string{"sre"},
		"iss":    "issuer",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := ActorFromContext(r.Context())
		if !ok || actor.ID != "user1" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", rw.Code)
	}
}

func TestAuthMiddlewareRejectsBadToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer badtoken")
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rw.Code)
	}
}

func TestAuthMiddlewareRejectsNonBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic abc")
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rw.Code)
	}
}

func TestActorFromContextMissing(t *testing.T) {
	if _, ok := ActorFromContext(context.Background()); ok {
		t.Fatalf("expected missing actor")
	}
}

func TestAuthMiddlewareRejectsIssuerMismatch(t *testing.T) {
	SetAuthConfig(AuthConfig{Issuer: "issuer"})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	token := tokenForClaims(t, map[string]any{
		"sub": "user1",
		"iss": "other",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rw.Code)
	}
}

func TestAuthMiddlewareRejectsAudienceMismatch(t *testing.T) {
	SetAuthConfig(AuthConfig{Audience: "client"})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	token := tokenForClaims(t, map[string]any{
		"sub": "user1",
		"aud": "other",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rw.Code)
	}
}

func TestAuthMiddlewareAcceptsIssuerAudience(t *testing.T) {
	SetAuthConfig(AuthConfig{Issuer: "issuer", Audience: "client", DevMode: true})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	token := tokenForClaims(t, map[string]any{
		"sub": "user1",
		"iss": "issuer",
		"aud": []string{"client", "other"},
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := ActorFromContext(r.Context())
		if !ok || actor.ID != "user1" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", rw.Code)
	}
}

func TestAudienceMatchesInvalidTypes(t *testing.T) {
	if audienceMatches(123, "client") {
		t.Fatalf("unexpected match")
	}
	if audienceMatches([]any{"other"}, "client") {
		t.Fatalf("unexpected match")
	}
	if !audienceMatches([]any{"client"}, "client") {
		t.Fatalf("expected match")
	}
	if !audienceMatches("client", "client") {
		t.Fatalf("expected match")
	}
	if audienceMatches([]string{"other"}, "client") {
		t.Fatalf("unexpected match")
	}
	if !audienceMatches([]string{"client"}, "client") {
		t.Fatalf("expected match")
	}
}

func TestAuthMiddlewareRejectsExpiredToken(t *testing.T) {
	oldTimeNow := auth.TimeNow
	auth.TimeNow = func() time.Time { return time.Unix(2000000000, 0) }
	t.Cleanup(func() { auth.TimeNow = oldTimeNow })

	token := tokenForClaims(t, map[string]any{
		"sub": "user1",
		"exp": 1000000000,
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rw.Code)
	}
}

func TestAuthMiddlewareRejectsNotYetValid(t *testing.T) {
	oldTimeNow := auth.TimeNow
	auth.TimeNow = func() time.Time { return time.Unix(1000000000, 0) }
	t.Cleanup(func() { auth.TimeNow = oldTimeNow })

	token := tokenForClaims(t, map[string]any{
		"sub": "user1",
		"nbf": 2000000000,
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rw.Code)
	}
}

func TestAuthMiddlewareAcceptsValidExp(t *testing.T) {
	SetAuthConfig(AuthConfig{DevMode: true})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	oldTimeNow := auth.TimeNow
	auth.TimeNow = func() time.Time { return time.Unix(1000000000, 0) }
	t.Cleanup(func() { auth.TimeNow = oldTimeNow })

	token := tokenForClaims(t, map[string]any{
		"sub": "user1",
		"exp": 2000000000,
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", rw.Code)
	}
}

func TestSetAuthConfigCopies(t *testing.T) {
	cfg := AuthConfig{Issuer: "iss", Audience: "aud", JWKSURL: "jwks"}
	SetAuthConfig(cfg)
	got := currentAuthConfig()
	if got.Issuer != "iss" || got.Audience != "aud" || !strings.Contains(got.JWKSURL, "jwks") {
		t.Fatalf("config: %#v", got)
	}
	SetAuthConfig(AuthConfig{DevMode: true})
}

func TestAuthMiddlewareRejectsInvalidSignature(t *testing.T) {
	key := newRSAKey(t)
	other := newRSAKey(t)
	token := signToken(t, other, "kid", map[string]any{"sub": "user1"})

	SetAuthConfig(AuthConfig{JWKSURL: "http://jwks"})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	oldFetch := auth.FetchJWKS
	auth.FetchJWKS = func(url string) (auth.JWKS, error) {
		return auth.JWKS{Keys: []auth.JWK{jwkForKey(key.PublicKey, "kid")}}, nil
	}
	t.Cleanup(func() { auth.FetchJWKS = oldFetch })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rw.Code)
	}
}

func TestAuthMiddlewareAcceptsSignedToken(t *testing.T) {
	key := newRSAKey(t)
	token := signToken(t, key, "kid", map[string]any{"sub": "user1"})

	SetAuthConfig(AuthConfig{JWKSURL: "http://jwks"})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	oldFetch := auth.FetchJWKS
	auth.FetchJWKS = func(url string) (auth.JWKS, error) {
		return auth.JWKS{Keys: []auth.JWK{jwkForKey(key.PublicKey, "kid")}}, nil
	}
	t.Cleanup(func() { auth.FetchJWKS = oldFetch })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", rw.Code)
	}
}

func TestAuthMiddlewareRejectsNoJWKSURL(t *testing.T) {
	SetAuthConfig(AuthConfig{})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	token := tokenForClaims(t, map[string]any{
		"sub": "user1",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d (SEC-02: empty JWKS URL must reject without DevMode)", rw.Code)
	}
}

func TestAuthMiddlewareAcceptsDevMode(t *testing.T) {
	SetAuthConfig(AuthConfig{DevMode: true})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	token := tokenForClaims(t, map[string]any{
		"sub":   "user1",
		"email": "dev@example.com",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := ActorFromContext(r.Context())
		if !ok || actor.ID != "user1" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("want 200 got %d (DevMode should skip signature verification)", rw.Code)
	}
}
