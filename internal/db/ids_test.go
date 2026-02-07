package db

import "testing"

func TestNewIDPrefix(t *testing.T) {
	id := newID("plan")
	if len(id) < 5 || id[:4] != "plan" {
		t.Fatalf("id: %s", id)
	}
}
