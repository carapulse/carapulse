package web

import "testing"

func TestParseJWTClaims(t *testing.T) {
	// header.payload.signature with base64url payload
	payload := "eyJzdWIiOiJ1c2VyMSIsImVtYWlsIjoidXNlckBleGFtcGxlLmNvbSIsImdyb3VwcyI6WyJzcmUiXSwiaXNzIjoiaXNzdWVyIn0"
	token := "aaa." + payload + ".bbb"
	claims, err := ParseJWTClaims(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != "user1" {
		t.Fatalf("sub: %s", claims.Sub)
	}
	if claims.Email != "user@example.com" {
		t.Fatalf("email: %s", claims.Email)
	}
	if len(claims.Groups) != 1 || claims.Groups[0] != "sre" {
		t.Fatalf("groups: %v", claims.Groups)
	}
}

func TestParseJWTClaimsInvalid(t *testing.T) {
	_, err := ParseJWTClaims("invalid")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseJWTClaimsBadBase64(t *testing.T) {
	_, err := ParseJWTClaims("aaa.bad!!.bbb")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseJWTClaimsBadJSON(t *testing.T) {
	payload := "eyJzdWIiOiJ1c2VyIg" // base64url of {"sub":"user" with truncation
	token := "aaa." + payload + ".bbb"
	_, err := ParseJWTClaims(token)
	if err == nil {
		t.Fatalf("expected error")
	}
}
