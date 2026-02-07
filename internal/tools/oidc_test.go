package tools

import (
	"net/http"
	"testing"
)

func TestParseBearerOK(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	token, err := ParseBearer(req)
	if err != nil || token != "token" {
		t.Fatalf("token: %s err: %v", token, err)
	}
}

func TestParseBearerMissing(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	if _, err := ParseBearer(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseBearerInvalid(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token abc")
	if _, err := ParseBearer(req); err == nil {
		t.Fatalf("expected error")
	}
}
