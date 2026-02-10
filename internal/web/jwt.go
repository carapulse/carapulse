package web

import (
	"time"

	"carapulse/internal/auth"
)

// Type aliases so existing code that references web.JWKS, web.JWK, etc.
// continues to compile without import changes.
type JWKS = auth.JWKS
type JWK = auth.JWK
type JWTPayload = auth.JWTPayload
type JWTHeader = auth.JWTHeader

func SetJWKSCacheTTL(ttl time.Duration) {
	auth.SetJWKSCacheTTL(ttl)
}

func ParseJWTClaims(token string) (JWTPayload, error) {
	return auth.ParseJWTClaims(token)
}

func ParseJWTHeader(token string) (JWTHeader, error) {
	return auth.ParseJWTHeader(token)
}

func VerifyJWTSignature(token string, cfg AuthConfig) error {
	return auth.VerifyJWTSignature(token, auth.AuthConfig{
		Issuer:   cfg.Issuer,
		Audience: cfg.Audience,
		JWKSURL:  cfg.JWKSURL,
		DevMode:  cfg.DevMode,
	})
}
