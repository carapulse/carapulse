package tools

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func encodeJSON(t *testing.T, payload any) string {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// TestParseJWTHeaderViaTools verifies the tools wrapper still works.
func TestParseJWTHeaderViaTools(t *testing.T) {
	header := encodeJSON(t, map[string]any{"alg": "RS256", "kid": "kid"})
	token := header + ".payload.sig"
	out, err := ParseJWTHeader(token)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.Alg != "RS256" || out.Kid != "kid" {
		t.Fatalf("header: %#v", out)
	}
}

// TestVerifyJWTSignatureNoJWKSURL verifies the tools wrapper behavior:
// tools.AuthConfig has no DevMode, so empty JWKS URL with default config
// should now return an error (security fix — auth.AuthConfig.DevMode=false).
func TestVerifyJWTSignatureNoJWKSURL(t *testing.T) {
	if err := VerifyJWTSignature("bad", AuthConfig{}); err == nil {
		t.Fatalf("expected error: empty JWKS URL without DevMode should reject")
	}
}

// TestVerifyJWTSignatureInvalidToken verifies error on bad token.
func TestVerifyJWTSignatureInvalidToken(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	if err := VerifyJWTSignature("bad", cfg); err == nil {
		t.Fatalf("expected error")
	}
}
