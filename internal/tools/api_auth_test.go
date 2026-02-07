package tools

import (
	"net/http"
	"testing"
)

func TestApplyAuth(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example", nil)
	ApplyAuth(req, AuthHeaders{BearerToken: "tok", BasicUser: "u", BasicPass: "p", Extra: map[string]string{"X-Test": "1"}})
	if req.Header.Get("Authorization") == "" {
		t.Fatalf("missing auth")
	}
	if req.Header.Get("X-Test") != "1" {
		t.Fatalf("missing extra header")
	}
}
