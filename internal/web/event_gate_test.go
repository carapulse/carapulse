package web

import (
	"context"
	"errors"
	"testing"
	"time"

	"carapulse/internal/db"
)

type fakeEventGateStore struct {
	allowed bool
	err     error
}

func (f *fakeEventGateStore) UpsertEventGate(ctx context.Context, source, fingerprint string, now time.Time, window, backoff time.Duration, minCount int) (bool, db.EventGateState, error) {
	return f.allowed, db.EventGateState{}, f.err
}

func TestEventGateAcceptNilGate(t *testing.T) {
	var g *EventGate
	allowed, _, err := g.Accept(context.Background(), "test", map[string]any{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed")
	}
}

func TestEventGateAcceptNilStore(t *testing.T) {
	g := &EventGate{}
	allowed, _, err := g.Accept(context.Background(), "test", map[string]any{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed")
	}
}

func TestEventGateAcceptAllowed(t *testing.T) {
	g := &EventGate{Store: &fakeEventGateStore{allowed: true}}
	allowed, fingerprint, err := g.Accept(context.Background(), "alertmanager", map[string]any{"status": "firing"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed")
	}
	if fingerprint == "" {
		t.Fatalf("expected fingerprint")
	}
}

func TestEventGateAcceptDenied(t *testing.T) {
	g := &EventGate{Store: &fakeEventGateStore{allowed: false}}
	allowed, _, err := g.Accept(context.Background(), "alertmanager", map[string]any{"status": "firing"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if allowed {
		t.Fatalf("expected denied")
	}
}

func TestEventGateAcceptStoreError(t *testing.T) {
	g := &EventGate{Store: &fakeEventGateStore{err: errors.New("boom")}}
	allowed, _, err := g.Accept(context.Background(), "alertmanager", map[string]any{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if allowed {
		t.Fatalf("expected denied on error")
	}
}

func TestEventGateAcceptSeverityFilter(t *testing.T) {
	g := &EventGate{
		Store:           &fakeEventGateStore{allowed: true},
		AllowSeverities: []string{"critical", "warning"},
	}
	// No severity in payload -> rejected
	allowed, _, err := g.Accept(context.Background(), "alertmanager", map[string]any{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if allowed {
		t.Fatalf("expected denied without severity")
	}

	// Severity not in allowlist -> rejected
	allowed, _, err = g.Accept(context.Background(), "alertmanager", map[string]any{
		"commonLabels": map[string]any{"severity": "info"},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if allowed {
		t.Fatalf("expected denied for info severity")
	}

	// Severity in allowlist -> accepted
	allowed, _, err = g.Accept(context.Background(), "alertmanager", map[string]any{
		"commonLabels": map[string]any{"severity": "critical"},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed for critical severity")
	}
}

func TestHashEvent(t *testing.T) {
	h1 := hashEvent("source1", map[string]any{"key": "val"})
	h2 := hashEvent("source2", map[string]any{"key": "val"})
	h3 := hashEvent("source1", map[string]any{"key": "val"})

	if h1 == "" {
		t.Fatalf("empty hash")
	}
	if h1 == h2 {
		t.Fatalf("different sources should produce different hashes")
	}
	if h1 != h3 {
		t.Fatalf("same inputs should produce same hash")
	}
}
