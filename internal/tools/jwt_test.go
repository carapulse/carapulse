package tools

import "testing"

// TestParseJWTClaimsViaTools verifies the tools wrapper still works.
func TestParseJWTClaimsViaTools(t *testing.T) {
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
	if _, err := ParseJWTClaims("invalid"); err == nil {
		t.Fatalf("expected error")
	}
}
