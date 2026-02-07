package tools

import (
	"context"
	"errors"
	"time"
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
