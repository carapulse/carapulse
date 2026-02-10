package web

import (
	"context"
	"net/http"
	"sync"

	"carapulse/internal/auth"
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
	DevMode  bool
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
		cfg := currentAuthConfig()
		if err := validateClaims(claims, cfg); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := VerifyJWTSignature(token, cfg); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		actor := Actor{ID: claims.Sub, Email: claims.Email, Roles: claims.Groups}
		r = r.WithContext(WithActor(r.Context(), actor))
		next.ServeHTTP(w, r)
	})
}

func validateClaims(claims JWTPayload, cfg AuthConfig) error {
	return auth.ValidateClaims(claims, auth.AuthConfig{
		Issuer:   cfg.Issuer,
		Audience: cfg.Audience,
		JWKSURL:  cfg.JWKSURL,
		DevMode:  cfg.DevMode,
	})
}

func audienceMatches(aud any, target string) bool {
	return auth.AudienceMatches(aud, target)
}
