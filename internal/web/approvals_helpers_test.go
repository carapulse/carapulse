package web

import "testing"

func TestNormalizeApprovalStatus(t *testing.T) {
	cases := []struct {
		input  string
		status string
		ok     bool
	}{
		{"", "pending", true},
		{"pending", "pending", true},
		{"approved", "approved", true},
		{"denied", "denied", true},
		{"expired", "expired", true},
		{"  APPROVED ", "approved", true},
		{"nope", "", false},
	}
	for _, c := range cases {
		status, ok := normalizeApprovalStatus(c.input)
		if status != c.status || ok != c.ok {
			t.Fatalf("input %q -> %q %v", c.input, status, ok)
		}
	}
}
