package web

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"
)

// Test helpers used by auth_test.go for AuthMiddleware integration tests.

func encodeJSON(t *testing.T, payload map[string]any) string {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func signToken(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	header := map[string]any{"alg": "RS256", "typ": "JWT"}
	if kid != "" {
		header["kid"] = kid
	}
	encHeader := encodeJSON(t, header)
	encPayload := encodeJSON(t, claims)
	signing := encHeader + "." + encPayload
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func newRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	return key
}

func jwkForKey(key rsa.PublicKey, kid string) JWK {
	return JWK{
		Kty: "RSA",
		Kid: kid,
		Alg: "RS256",
		N:   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
}

// TestParseJWTClaimsViaWeb verifies the web wrapper still works.
func TestParseJWTClaimsViaWeb(t *testing.T) {
	payload := "eyJzdWIiOiJ1c2VyMSIsImVtYWlsIjoidXNlckBleGFtcGxlLmNvbSIsImdyb3VwcyI6WyJzcmUiXSwiaXNzIjoiaXNzdWVyIn0"
	token := "aaa." + payload + ".bbb"
	claims, err := ParseJWTClaims(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != "user1" {
		t.Fatalf("sub: %s", claims.Sub)
	}
}

// TestParseJWTHeaderViaWeb verifies the web wrapper still works.
func TestParseJWTHeaderViaWeb(t *testing.T) {
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

// TestVerifyJWTSignatureViaWeb verifies the web wrapper delegates correctly.
func TestVerifyJWTSignatureViaWeb(t *testing.T) {
	if err := VerifyJWTSignature("bad", AuthConfig{}); err == nil {
		t.Fatalf("expected error when JWKS URL empty and DevMode false")
	}
	if err := VerifyJWTSignature("bad", AuthConfig{DevMode: true}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
