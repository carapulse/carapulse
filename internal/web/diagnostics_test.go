package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"carapulse/internal/tools"
)

type diagStore struct {
	keys []string
	data [][]byte
}

func (d *diagStore) Put(ctx context.Context, key string, data []byte) (string, error) {
	d.keys = append(d.keys, key)
	d.data = append(d.data, data)
	return "ref-" + key, nil
}

func (d *diagStore) Presign(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "signed-" + key, nil
}

func TestToolDiagnosticsCollect(t *testing.T) {
	var calls []tools.ExecuteRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req tools.ExecuteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		calls = append(calls, req)
		_ = json.NewEncoder(w).Encode(tools.ExecuteResponse{ToolCallID: "1", Output: []byte(`{"ok":true}`)})
	}))
	defer srv.Close()

	store := &diagStore{}
	d := &ToolDiagnostics{
		Router:     &tools.RouterClient{BaseURL: srv.URL, HTTPClient: srv.Client()},
		Store:      store,
		PresignTTL: time.Minute,
		Now:        func() time.Time { return time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC) },
	}
	ctxRef := ContextRef{
		TenantID:      "t",
		Environment:   "env",
		ClusterID:     "c",
		Namespace:     "ns",
		AWSAccountID:  "a",
		Region:        "r",
		ArgoCDProject: "p",
		GrafanaOrgID:  "g",
	}
	out, err := d.Collect(context.Background(), ctxRef, "latency spike", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("evidence: %d", len(out))
	}
	if len(store.keys) != 3 || len(calls) != 3 {
		t.Fatalf("calls=%d keys=%d", len(calls), len(store.keys))
	}
}

func TestDiagnosticsHelpers(t *testing.T) {
	d := &ToolDiagnostics{Now: func() time.Time { return time.Unix(0, 0) }}
	if ref, link := d.storeDiagnostic(context.Background(), "prom", "query", nil); ref != "" || link != "" {
		t.Fatalf("expected empty")
	}
	if query := queryFromIntent(""); query != "up" {
		t.Fatalf("query: %s", query)
	}
	if query := queryFromIntent("latency spike"); query == "up" {
		t.Fatalf("expected latency query")
	}
	if query := queryFromIntent("error rate"); query == "up" {
		t.Fatalf("expected error query")
	}
	toolCtx := toToolContext(ContextRef{TenantID: "t", Environment: "e"})
	if toolCtx.TenantID != "t" || toolCtx.Environment != "e" {
		t.Fatalf("tool ctx: %#v", toolCtx)
	}
	if _, err := diagnosticsFromJSON([]byte("{")); err == nil {
		t.Fatalf("expected error")
	}
	if items, err := diagnosticsFromJSON(nil); err != nil || items != nil {
		t.Fatalf("expected nil")
	}
	if items, err := diagnosticsFromJSON([]byte(`[{"type":"promql","query":"up"}]`)); err != nil || len(items) != 1 {
		t.Fatalf("items: %#v err=%v", items, err)
	}
}

func TestDiagnosticsHintsFromConstraints(t *testing.T) {
	constraints := map[string]any{
		"diagnostic_hints": map[string]any{
			"promql":    []any{"up", "up"},
			"traceql":   "span.error",
			"trace_ids": []any{"abc"},
		},
	}
	prom, trace, ids := diagnosticHintsFromConstraints(constraints, "intent")
	if len(prom) != 1 || prom[0] != "up" {
		t.Fatalf("promql: %#v", prom)
	}
	if len(trace) != 1 || trace[0] != "span.error" {
		t.Fatalf("traceql: %#v", trace)
	}
	if len(ids) != 1 || ids[0] != "abc" {
		t.Fatalf("trace_ids: %#v", ids)
	}
}
