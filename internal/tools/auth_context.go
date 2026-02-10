package tools

import (
	"context"

	"carapulse/internal/auth"
)

type ctxKey string

const (
	actorIDKey     ctxKey = "actor_id"
	actorEmailKey  ctxKey = "actor_email"
	actorRolesKey  ctxKey = "actor_roles"
	actorTenantKey ctxKey = "actor_tenant_id"
	sessionIDKey   ctxKey = "session_id"
)

func contextWithActor(ctx context.Context, claims JWTPayload) context.Context {
	ctx = context.WithValue(ctx, actorIDKey, claims.Sub)
	ctx = context.WithValue(ctx, actorEmailKey, claims.Email)
	ctx = context.WithValue(ctx, actorRolesKey, claims.Groups)
	ctx = context.WithValue(ctx, actorTenantKey, claims.TenantID)
	return ctx
}

func validateClaims(claims JWTPayload, cfg AuthConfig) error {
	return auth.ValidateClaims(claims, auth.AuthConfig{
		Issuer:   cfg.Issuer,
		Audience: cfg.Audience,
		JWKSURL:  cfg.JWKSURL,
		Token:    cfg.Token,
	})
}

func audienceMatches(aud any, target string) bool {
	return auth.AudienceMatches(aud, target)
}
