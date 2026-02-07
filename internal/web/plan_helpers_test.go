package web

import "testing"

func TestIsWriteAction(t *testing.T) {
	cases := []struct {
		intent string
		want   bool
	}{
		{"deploy service", true},
		{"scale api", true},
		{"rollback release", true},
		{"sync argo", true},
		{"delete user", true},
		{"show status", false},
	}
	for _, c := range cases {
		got := isWriteAction(c.intent)
		if got != c.want {
			t.Fatalf("intent %q: want %v got %v", c.intent, c.want, got)
		}
	}
}

func TestRiskFromIntent(t *testing.T) {
	cases := []struct {
		intent string
		want   string
	}{
		{"deploy service", "low"},
		{"scale api", "low"},
		{"rollback release", "low"},
		{"sync argo", "medium"},
		{"restart deployment", "medium"},
		{"delete user", "high"},
		{"update iam role", "high"},
		{"show status", "read"},
	}
	for _, c := range cases {
		got := riskFromIntent(c.intent)
		if got != c.want {
			t.Fatalf("intent %q: want %s got %s", c.intent, c.want, got)
		}
	}
}
