package tools

import "testing"

func TestAllowHostExactAndWildcard(t *testing.T) {
	if !allowHost("https://example.com", []string{"example.com"}) {
		t.Fatalf("expected allow")
	}
	if !allowHost("https://api.example.com", []string{"*.example.com"}) {
		t.Fatalf("expected wildcard allow")
	}
	if allowHost("https://example.com", []string{"*.other.com"}) {
		t.Fatalf("unexpected allow")
	}
}

func TestAllowHostInvalid(t *testing.T) {
	if allowHost("http://%", []string{"example.com"}) {
		t.Fatalf("unexpected allow")
	}
	if allowHost("http://", []string{"example.com"}) {
		t.Fatalf("unexpected allow")
	}
}
