package tools

import (
	"testing"
	"time"

	"carapulse/internal/auth"
)

func TestValidateClaims(t *testing.T) {
	claims := JWTPayload{Iss: "iss", Aud: "aud"}
	if err := validateClaims(claims, AuthConfig{Issuer: "iss", Audience: "aud"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := validateClaims(claims, AuthConfig{Issuer: "bad"}); err == nil {
		t.Fatalf("expected error")
	}
	if err := validateClaims(claims, AuthConfig{Audience: "bad"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateClaimsExpired(t *testing.T) {
	oldTimeNow := auth.TimeNow
	auth.TimeNow = func() time.Time { return time.Unix(2000000000, 0) }
	t.Cleanup(func() { auth.TimeNow = oldTimeNow })

	claims := JWTPayload{Iss: "iss", Aud: "aud", Exp: 1000000000}
	if err := validateClaims(claims, AuthConfig{Issuer: "iss", Audience: "aud"}); err == nil {
		t.Fatalf("expected error for expired token")
	}
}

func TestValidateClaimsNotYetValid(t *testing.T) {
	oldTimeNow := auth.TimeNow
	auth.TimeNow = func() time.Time { return time.Unix(1000000000, 0) }
	t.Cleanup(func() { auth.TimeNow = oldTimeNow })

	claims := JWTPayload{Iss: "iss", Aud: "aud", Nbf: 2000000000}
	if err := validateClaims(claims, AuthConfig{Issuer: "iss", Audience: "aud"}); err == nil {
		t.Fatalf("expected error for not-yet-valid token")
	}
}

func TestValidateClaimsValidExp(t *testing.T) {
	oldTimeNow := auth.TimeNow
	auth.TimeNow = func() time.Time { return time.Unix(1000000000, 0) }
	t.Cleanup(func() { auth.TimeNow = oldTimeNow })

	claims := JWTPayload{Iss: "iss", Aud: "aud", Exp: 2000000000}
	if err := validateClaims(claims, AuthConfig{Issuer: "iss", Audience: "aud"}); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAudienceMatches(t *testing.T) {
	if !audienceMatches("aud", "aud") {
		t.Fatalf("string")
	}
	if !audienceMatches([]string{"x", "aud"}, "aud") {
		t.Fatalf("slice")
	}
	if !audienceMatches([]any{"aud"}, "aud") {
		t.Fatalf("any slice")
	}
	if audienceMatches([]any{123}, "aud") {
		t.Fatalf("unexpected match")
	}
}
