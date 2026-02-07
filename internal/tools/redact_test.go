package tools

import "testing"

func TestRedactorNilPatterns(t *testing.T) {
	if got := NewRedactor(nil); got != nil {
		t.Fatalf("expected nil")
	}
	if got := NewRedactor([]string{""}); got != nil {
		t.Fatalf("expected nil")
	}
}

func TestRedactorRedacts(t *testing.T) {
	r := NewRedactor([]string{"secret=\\w+"})
	if r == nil {
		t.Fatalf("expected redactor")
	}
	out := r.Redact([]byte("secret=abc"))
	if string(out) != "***" {
		t.Fatalf("out: %s", string(out))
	}
	str := r.RedactString("token=123 secret=xyz")
	if str != "token=123 ***" {
		t.Fatalf("str: %s", str)
	}
}
