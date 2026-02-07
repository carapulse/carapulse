package tools

import (
	"context"
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"hash"
	"math/big"
	"strings"
	"time"

	"carapulse/internal/auth"
)

type JWKS = auth.JWKS
type JWK = auth.JWK

var jwksCache = auth.NewJWKSCache(time.Hour)
var fetchJWKS = func(url string) (JWKS, error) {
	return jwksCache.Get(context.Background(), url)
}
var hashWrite = func(h hash.Hash, data []byte) error {
	_, err := h.Write(data)
	return err
}

func SetJWKSCacheTTL(ttl time.Duration) {
	jwksCache.Close()
	jwksCache = auth.NewJWKSCache(ttl)
	fetchJWKS = func(url string) (JWKS, error) {
		return jwksCache.Get(context.Background(), url)
	}
}

type JWTPayload struct {
	Sub      string   `json:"sub"`
	Email    string   `json:"email"`
	Groups   []string `json:"groups"`
	Iss      string   `json:"iss"`
	Aud      any      `json:"aud"`
	Exp      float64  `json:"exp"`
	Nbf      float64  `json:"nbf"`
	TenantID string   `json:"tenant_id"`
}

type JWTHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

func ParseJWTClaims(token string) (JWTPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return JWTPayload{}, errors.New("invalid jwt")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return JWTPayload{}, err
	}
	var claims JWTPayload
	if err := json.Unmarshal(payload, &claims); err != nil {
		return JWTPayload{}, err
	}
	return claims, nil
}

func ParseJWTHeader(token string) (JWTHeader, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return JWTHeader{}, errors.New("invalid jwt")
	}
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return JWTHeader{}, err
	}
	var out JWTHeader
	if err := json.Unmarshal(header, &out); err != nil {
		return JWTHeader{}, err
	}
	return out, nil
}

func VerifyJWTSignature(token string, cfg AuthConfig) error {
	jwksURL := strings.TrimSpace(cfg.JWKSURL)
	if jwksURL == "" {
		return nil
	}
	header, signed, sig, err := parseJWTForVerify(token)
	if err != nil {
		return err
	}
	if header.Alg == "" {
		return errors.New("jwt alg required")
	}
	if header.Alg != "RS256" {
		return errors.New("unsupported jwt alg")
	}
	jwks, err := fetchJWKS(jwksURL)
	if err != nil {
		return err
	}
	key, err := selectJWK(jwks.Keys, header.Kid)
	if err != nil {
		return err
	}
	pub, err := rsaKeyFromJWK(key)
	if err != nil {
		return err
	}
	return verifyRS256(signed, sig, pub)
}

func parseJWTForVerify(token string) (JWTHeader, []byte, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return JWTHeader{}, nil, nil, errors.New("invalid jwt")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return JWTHeader{}, nil, nil, err
	}
	var header JWTHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return JWTHeader{}, nil, nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return JWTHeader{}, nil, nil, err
	}
	signed := []byte(parts[0] + "." + parts[1])
	return header, signed, sig, nil
}

func selectJWK(keys []JWK, kid string) (JWK, error) {
	if kid != "" {
		for _, key := range keys {
			if key.Kid == kid {
				return key, nil
			}
		}
		return JWK{}, errors.New("jwks kid not found")
	}
	if len(keys) == 1 {
		return keys[0], nil
	}
	return JWK{}, errors.New("jwks kid required")
}

func rsaKeyFromJWK(key JWK) (*rsa.PublicKey, error) {
	if key.Kty != "" && key.Kty != "RSA" {
		return nil, errors.New("unsupported jwk kty")
	}
	if key.N == "" || key.E == "" {
		return nil, errors.New("invalid jwk")
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, err
	}
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	if !e.IsInt64() {
		return nil, errors.New("invalid jwk exponent")
	}
	exp := int(e.Int64())
	if exp <= 0 {
		return nil, errors.New("invalid jwk exponent")
	}
	return &rsa.PublicKey{N: n, E: exp}, nil
}

func verifyRS256(signed []byte, sig []byte, key *rsa.PublicKey) error {
	sum := crypto.SHA256.New()
	if err := hashWrite(sum, signed); err != nil {
		return err
	}
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, sum.Sum(nil), sig)
}
