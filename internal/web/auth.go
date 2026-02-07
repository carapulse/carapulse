package web

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

type Actor struct {
	ID    string
	Email string
	Roles []string
}

type actorKey struct{}

type AuthConfig struct {
	Issuer   string
	Audience string
	JWKSURL  string
}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorKey{}).(Actor)
	return actor, ok
}

var authConfig AuthConfig
var authMu sync.RWMutex

func SetAuthConfig(cfg AuthConfig) {
	authMu.Lock()
	authConfig = cfg
	authMu.Unlock()
}

func currentAuthConfig() AuthConfig {
	authMu.RLock()
	defer authMu.RUnlock()
	return authConfig
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := ParseBearer(r)
		if err != nil || token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		claims, err := ParseJWTClaims(token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := validateClaims(claims, currentAuthConfig()); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := VerifyJWTSignature(token, currentAuthConfig()); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		actor := Actor{ID: claims.Sub, Email: claims.Email, Roles: claims.Groups}
		r = r.WithContext(WithActor(r.Context(), actor))
		next.ServeHTTP(w, r)
	})
}

var timeNow = time.Now

func validateClaims(claims JWTPayload, cfg AuthConfig) error {
	if cfg.Issuer != "" && claims.Iss != cfg.Issuer {
		return errors.New("issuer mismatch")
	}
	if cfg.Audience != "" && !audienceMatches(claims.Aud, cfg.Audience) {
		return errors.New("audience mismatch")
	}
	now := float64(timeNow().Unix())
	if claims.Exp > 0 && now >= claims.Exp {
		return errors.New("token expired")
	}
	if claims.Nbf > 0 && now < claims.Nbf {
		return errors.New("token not yet valid")
	}
	return nil
}

func audienceMatches(aud any, target string) bool {
	switch v := aud.(type) {
	case string:
		return v == target
	case []string:
		for _, item := range v {
			if item == target {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == target {
				return true
			}
		}
	}
	return false
}
