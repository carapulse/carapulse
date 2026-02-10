package auth

import (
	"errors"
	"time"
)

// TimeNow is the function used to get the current time. Package-level var for
// test injection.
var TimeNow = time.Now

// ValidateClaims checks issuer, audience, expiration, and not-before claims.
func ValidateClaims(claims JWTPayload, cfg AuthConfig) error {
	if cfg.Issuer != "" && claims.Iss != cfg.Issuer {
		return errors.New("issuer mismatch")
	}
	if cfg.Audience != "" && !AudienceMatches(claims.Aud, cfg.Audience) {
		return errors.New("audience mismatch")
	}
	now := float64(TimeNow().Unix())
	if claims.Exp > 0 && now >= claims.Exp {
		return errors.New("token expired")
	}
	if claims.Nbf > 0 && now < claims.Nbf {
		return errors.New("token not yet valid")
	}
	return nil
}

// AudienceMatches checks whether aud (string, []string, or []any) contains target.
func AudienceMatches(aud any, target string) bool {
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
