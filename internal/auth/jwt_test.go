package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"hash"
	"math/big"
	"net/http"
	"testing"
	"time"
)

func encodeJSON(t *testing.T, payload any) string {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func newRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	return key
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

func jwkForKey(key rsa.PublicKey, kid string) JWK {
	return JWK{
		Kty: "RSA",
		Kid: kid,
		Alg: "RS256",
		N:   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
}

// --- ParseJWTClaims tests ---

func TestParseJWTClaims(t *testing.T) {
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

func TestParseJWTClaimsBadBase64(t *testing.T) {
	if _, err := ParseJWTClaims("aaa.bad!!.bbb"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseJWTClaimsBadJSON(t *testing.T) {
	payload := "eyJzdWIiOiJ1c2VyIg"
	token := "aaa." + payload + ".bbb"
	if _, err := ParseJWTClaims(token); err == nil {
		t.Fatalf("expected error")
	}
}

// --- ParseJWTHeader tests ---

func TestParseJWTHeaderOK(t *testing.T) {
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

func TestParseJWTHeaderInvalid(t *testing.T) {
	if _, err := ParseJWTHeader("invalid"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseJWTHeaderBadBase64(t *testing.T) {
	if _, err := ParseJWTHeader("bad!!.payload.sig"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseJWTHeaderBadJSON(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte("{"))
	if _, err := ParseJWTHeader(header + ".payload.sig"); err == nil {
		t.Fatalf("expected error")
	}
}

// --- VerifyJWTSignature tests ---

func TestVerifyJWTSignatureNoJWKSURLRejects(t *testing.T) {
	if err := VerifyJWTSignature("bad", AuthConfig{}); err == nil {
		t.Fatalf("expected error when JWKS URL empty and DevMode false")
	}
}

func TestVerifyJWTSignatureNoJWKSURLDevMode(t *testing.T) {
	if err := VerifyJWTSignature("bad", AuthConfig{DevMode: true}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestVerifyJWTSignatureInvalidToken(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	if err := VerifyJWTSignature("bad", cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureBadHeaderBase64(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	token := "bad!!.payload.sig"
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureBadHeaderJSON(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	header := base64.RawURLEncoding.EncodeToString([]byte("{"))
	token := header + ".payload.sig"
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureBadSigBase64(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	header := encodeJSON(t, map[string]any{"alg": "RS256"})
	token := header + ".payload.bad!!"
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureMissingAlg(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	header := encodeJSON(t, map[string]any{"kid": "kid"})
	token := header + ".payload.sig"
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureUnsupportedAlg(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	header := encodeJSON(t, map[string]any{"alg": "HS256"})
	token := header + ".payload.sig"
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureFetchError(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	header := encodeJSON(t, map[string]any{"alg": "RS256"})
	token := header + ".payload.sig"
	oldFetch := FetchJWKS
	FetchJWKS = func(url string) (JWKS, error) { return JWKS{}, errors.New("boom") }
	t.Cleanup(func() { FetchJWKS = oldFetch })
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureKidNotFound(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	header := encodeJSON(t, map[string]any{"alg": "RS256", "kid": "kid-1"})
	token := header + ".payload.sig"
	oldFetch := FetchJWKS
	FetchJWKS = func(url string) (JWKS, error) {
		return JWKS{Keys: []JWK{{Kid: "kid-2", Kty: "RSA", N: "AA", E: "AQAB"}}}, nil
	}
	t.Cleanup(func() { FetchJWKS = oldFetch })
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureKidRequired(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	header := encodeJSON(t, map[string]any{"alg": "RS256"})
	token := header + ".payload.sig"
	oldFetch := FetchJWKS
	FetchJWKS = func(url string) (JWKS, error) {
		return JWKS{Keys: []JWK{
			{Kid: "kid-1", Kty: "RSA", N: "AA", E: "AQAB"},
			{Kid: "kid-2", Kty: "RSA", N: "AA", E: "AQAB"},
		}}, nil
	}
	t.Cleanup(func() { FetchJWKS = oldFetch })
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureBadKty(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	header := encodeJSON(t, map[string]any{"alg": "RS256", "kid": "kid"})
	token := header + ".payload.sig"
	oldFetch := FetchJWKS
	FetchJWKS = func(url string) (JWKS, error) {
		return JWKS{Keys: []JWK{{Kid: "kid", Kty: "EC", N: "AA", E: "AQAB"}}}, nil
	}
	t.Cleanup(func() { FetchJWKS = oldFetch })
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureBadN(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	header := encodeJSON(t, map[string]any{"alg": "RS256", "kid": "kid"})
	token := header + ".payload.sig"
	oldFetch := FetchJWKS
	FetchJWKS = func(url string) (JWKS, error) {
		return JWKS{Keys: []JWK{{Kid: "kid", Kty: "RSA", N: "bad!!", E: "AQAB"}}}, nil
	}
	t.Cleanup(func() { FetchJWKS = oldFetch })
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureBadE(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	header := encodeJSON(t, map[string]any{"alg": "RS256", "kid": "kid"})
	token := header + ".payload.sig"
	oldFetch := FetchJWKS
	FetchJWKS = func(url string) (JWKS, error) {
		return JWKS{Keys: []JWK{{Kid: "kid", Kty: "RSA", N: "AA", E: "bad!!"}}}, nil
	}
	t.Cleanup(func() { FetchJWKS = oldFetch })
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureBadExponent(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	header := encodeJSON(t, map[string]any{"alg": "RS256", "kid": "kid"})
	token := header + ".payload.sig"
	oldFetch := FetchJWKS
	FetchJWKS = func(url string) (JWKS, error) {
		return JWKS{Keys: []JWK{{Kid: "kid", Kty: "RSA", N: "AA", E: "AA"}}}, nil
	}
	t.Cleanup(func() { FetchJWKS = oldFetch })
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureBadSignature(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	key := newRSAKey(t)
	otherKey := newRSAKey(t)
	token := signToken(t, otherKey, "kid", map[string]any{"sub": "user"})
	oldFetch := FetchJWKS
	FetchJWKS = func(url string) (JWKS, error) {
		return JWKS{Keys: []JWK{jwkForKey(key.PublicKey, "kid")}}, nil
	}
	t.Cleanup(func() { FetchJWKS = oldFetch })
	if err := VerifyJWTSignature(token, cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyJWTSignatureOK(t *testing.T) {
	cfg := AuthConfig{JWKSURL: "http://jwks"}
	key := newRSAKey(t)
	token := signToken(t, key, "kid", map[string]any{"sub": "user"})
	oldFetch := FetchJWKS
	FetchJWKS = func(url string) (JWKS, error) {
		return JWKS{Keys: []JWK{jwkForKey(key.PublicKey, "kid")}}, nil
	}
	t.Cleanup(func() { FetchJWKS = oldFetch })
	if err := VerifyJWTSignature(token, cfg); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestSelectJWKSingleNoKid(t *testing.T) {
	key, err := SelectJWK([]JWK{{Kid: "kid", Kty: "RSA", N: "AA", E: "AQAB"}}, "")
	if err != nil || key.Kid != "kid" {
		t.Fatalf("unexpected: %v %#v", err, key)
	}
}

func TestRSAKeyFromJWKMissingFields(t *testing.T) {
	if _, err := RSAKeyFromJWK(JWK{Kty: "RSA", N: "", E: "AQAB"}); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := RSAKeyFromJWK(JWK{Kty: "RSA", N: "AA", E: ""}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRSAKeyFromJWKExponentTooLarge(t *testing.T) {
	tooLarge := new(big.Int).Lsh(big.NewInt(1), 63)
	enc := base64.RawURLEncoding.EncodeToString(tooLarge.Bytes())
	if _, err := RSAKeyFromJWK(JWK{Kty: "RSA", N: "AA", E: enc}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestVerifyRS256HashWriteError(t *testing.T) {
	oldWrite := HashWrite
	HashWrite = func(h hash.Hash, data []byte) error { return errors.New("write") }
	t.Cleanup(func() { HashWrite = oldWrite })

	key := newRSAKey(t)
	if err := VerifyRS256([]byte("data"), []byte("sig"), &key.PublicKey); err == nil {
		t.Fatalf("expected error")
	}
}

// --- ValidateClaims tests ---

func TestValidateClaims(t *testing.T) {
	claims := JWTPayload{Iss: "iss", Aud: "aud"}
	if err := ValidateClaims(claims, AuthConfig{Issuer: "iss", Audience: "aud"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := ValidateClaims(claims, AuthConfig{Issuer: "bad"}); err == nil {
		t.Fatalf("expected error")
	}
	if err := ValidateClaims(claims, AuthConfig{Audience: "bad"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateClaimsExpired(t *testing.T) {
	oldTimeNow := TimeNow
	TimeNow = func() time.Time { return time.Unix(2000000000, 0) }
	t.Cleanup(func() { TimeNow = oldTimeNow })

	claims := JWTPayload{Iss: "iss", Aud: "aud", Exp: 1000000000}
	if err := ValidateClaims(claims, AuthConfig{Issuer: "iss", Audience: "aud"}); err == nil {
		t.Fatalf("expected error for expired token")
	}
}

func TestValidateClaimsNotYetValid(t *testing.T) {
	oldTimeNow := TimeNow
	TimeNow = func() time.Time { return time.Unix(1000000000, 0) }
	t.Cleanup(func() { TimeNow = oldTimeNow })

	claims := JWTPayload{Iss: "iss", Aud: "aud", Nbf: 2000000000}
	if err := ValidateClaims(claims, AuthConfig{Issuer: "iss", Audience: "aud"}); err == nil {
		t.Fatalf("expected error for not-yet-valid token")
	}
}

func TestValidateClaimsValidExp(t *testing.T) {
	oldTimeNow := TimeNow
	TimeNow = func() time.Time { return time.Unix(1000000000, 0) }
	t.Cleanup(func() { TimeNow = oldTimeNow })

	claims := JWTPayload{Iss: "iss", Aud: "aud", Exp: 2000000000}
	if err := ValidateClaims(claims, AuthConfig{Issuer: "iss", Audience: "aud"}); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAudienceMatches(t *testing.T) {
	if !AudienceMatches("aud", "aud") {
		t.Fatalf("string")
	}
	if !AudienceMatches([]string{"x", "aud"}, "aud") {
		t.Fatalf("slice")
	}
	if !AudienceMatches([]any{"aud"}, "aud") {
		t.Fatalf("any slice")
	}
	if AudienceMatches([]any{123}, "aud") {
		t.Fatalf("unexpected match")
	}
	if AudienceMatches(123, "aud") {
		t.Fatalf("unexpected match")
	}
}

// --- ParseBearer tests ---

func TestParseBearerMissing(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	if _, err := ParseBearer(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseBearerInvalid(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic abc")
	if _, err := ParseBearer(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseBearerValid(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer my-token")
	tok, err := ParseBearer(req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tok != "my-token" {
		t.Fatalf("token: %s", tok)
	}
}

func TestParseBearerCaseInsensitive(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "BEARER my-token")
	tok, err := ParseBearer(req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tok != "my-token" {
		t.Fatalf("token: %s", tok)
	}
}
