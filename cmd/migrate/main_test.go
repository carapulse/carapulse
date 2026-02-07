package main

import "testing"

func TestRunMissingDSN(t *testing.T) {
	if err := run([]string{"-action", "up"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunMissingAction(t *testing.T) {
	if err := run([]string{"-dsn", "postgres://example"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunUnknownAction(t *testing.T) {
	if err := run([]string{"-dsn", "postgres://example", "-action", "nope"}); err == nil {
		t.Fatalf("expected error")
	}
}

