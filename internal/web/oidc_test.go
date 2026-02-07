package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := ParseBearer(req); err == nil {
		t.Fatalf("expected error")
	}
	req.Header.Set("Authorization", "Bearer token")
	if tok, err := ParseBearer(req); err != nil || tok != "token" {
		t.Fatalf("tok: %v %s", err, tok)
	}
}
