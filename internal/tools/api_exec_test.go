package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecuteAPIPrometheusActions(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	router := NewRouter()
	clients := HTTPClients{Prometheus: &APIClient{BaseURL: srv.URL}}
	if out, err := router.ExecuteAPI(context.Background(), "prometheus", "query", map[string]any{"query": "up"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("query: %v %s", err, string(out))
	}
	if gotPath != "/api/v1/query" || gotMethod != http.MethodPost {
		t.Fatalf("path: %s method: %s", gotPath, gotMethod)
	}
	if out, err := router.ExecuteAPI(context.Background(), "prometheus", "query_range", map[string]any{"query": "up"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("range: %v %s", err, string(out))
	}
	if gotPath != "/api/v1/query_range" {
		t.Fatalf("range path: %s", gotPath)
	}
	if out, err := router.ExecuteAPI(context.Background(), "prometheus", "rules", nil, clients); err != nil || string(out) != "ok" {
		t.Fatalf("rules: %v %s", err, string(out))
	}
	if gotPath != "/api/v1/rules" || gotMethod != http.MethodGet {
		t.Fatalf("rules path: %s method: %s", gotPath, gotMethod)
	}
	if out, err := router.ExecuteAPI(context.Background(), "prometheus", "query_exemplars", map[string]any{"query": "up"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("exemplars: %v %s", err, string(out))
	}
	if gotPath != "/api/v1/query_exemplars" {
		t.Fatalf("exemplars path: %s", gotPath)
	}
}

func TestExecuteAPIThanosParams(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	router := NewRouter()
	clients := HTTPClients{Thanos: &APIClient{BaseURL: srv.URL}}
	out, err := router.ExecuteAPI(context.Background(), "thanos", "query_range", map[string]any{
		"query":                 "up",
		"dedup":                 true,
		"max_source_resolution": "5m",
	}, clients)
	if err != nil || string(out) != "ok" {
		t.Fatalf("err: %v %s", err, string(out))
	}
	if gotQuery == "" {
		t.Fatalf("missing query")
	}
}

func TestExecuteAPIGrafanaActions(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	router := NewRouter()
	clients := HTTPClients{Grafana: &APIClient{BaseURL: srv.URL}}
	if out, err := router.ExecuteAPI(context.Background(), "grafana", "annotate", map[string]any{"text": "x"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("annotate: %v %s", err, string(out))
	}
	if gotPath != "/api/annotations" || gotMethod != http.MethodPost {
		t.Fatalf("annotate path: %s method: %s", gotPath, gotMethod)
	}
	if out, err := router.ExecuteAPI(context.Background(), "grafana", "dashboard_get", map[string]any{"uid": "abc"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("get: %v %s", err, string(out))
	}
	if gotPath != "/api/dashboards/uid/abc" || gotMethod != http.MethodGet {
		t.Fatalf("get path: %s method: %s", gotPath, gotMethod)
	}
	if out, err := router.ExecuteAPI(context.Background(), "grafana", "dashboard_upsert", map[string]any{"dashboard": map[string]any{"id": 1}}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("upsert: %v %s", err, string(out))
	}
	if gotPath != "/api/dashboards/db" || gotMethod != http.MethodPost {
		t.Fatalf("upsert path: %s method: %s", gotPath, gotMethod)
	}
	if out, err := router.ExecuteAPI(context.Background(), "grafana", "dashboard_list", map[string]any{"query": "svc", "folder_ids": []any{1}}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("list: %v %s", err, string(out))
	}
	if gotPath != "/api/search" || gotMethod != http.MethodGet {
		t.Fatalf("list path: %s method: %s", gotPath, gotMethod)
	}
	if out, err := router.ExecuteAPI(context.Background(), "grafana", "dashboard_delete", map[string]any{"uid": "abc"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("delete: %v %s", err, string(out))
	}
	if gotPath != "/api/dashboards/uid/abc" || gotMethod != http.MethodDelete {
		t.Fatalf("delete path: %s method: %s", gotPath, gotMethod)
	}
}

func TestExecuteAPITempoActions(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	router := NewRouter()
	clients := HTTPClients{Tempo: &APIClient{BaseURL: srv.URL}}
	if out, err := router.ExecuteAPI(context.Background(), "tempo", "traceql", map[string]any{"query": "span"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("traceql: %v %s", err, string(out))
	}
	if gotPath != "/api/search" || gotMethod != http.MethodPost {
		t.Fatalf("traceql path: %s method: %s", gotPath, gotMethod)
	}
	if out, err := router.ExecuteAPI(context.Background(), "tempo", "trace_by_id", map[string]any{"trace_id": "abc"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("trace: %v %s", err, string(out))
	}
	if gotPath != "/api/v2/traces/abc" || gotMethod != http.MethodGet {
		t.Fatalf("trace path: %s method: %s", gotPath, gotMethod)
	}
}

func TestExecuteAPIVaultBoundaryArgoAWS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	router := NewRouter()
	clients := HTTPClients{
		Vault:    &APIClient{BaseURL: srv.URL},
		Boundary: &APIClient{BaseURL: srv.URL},
		ArgoCD:   &APIClient{BaseURL: srv.URL},
		AWS:      &APIClient{BaseURL: srv.URL},
	}
	if out, err := router.ExecuteAPI(context.Background(), "vault", "login_k8s", map[string]any{"jwt": "x"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("vault: %v %s", err, string(out))
	}
	if out, err := router.ExecuteAPI(context.Background(), "boundary", "session_open", map[string]any{"target_id": "t"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("boundary: %v %s", err, string(out))
	}
	if out, err := router.ExecuteAPI(context.Background(), "boundary", "session_close", map[string]any{"session_id": "s"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("boundary close: %v %s", err, string(out))
	}
	if out, err := router.ExecuteAPI(context.Background(), "argocd", "status", map[string]any{"app": "app"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("argocd: %v %s", err, string(out))
	}
	if out, err := router.ExecuteAPI(context.Background(), "aws", "cloudtrail_lookup_events", map[string]any{"lookup": "x"}, clients); err != nil || string(out) != "ok" {
		t.Fatalf("aws: %v %s", err, string(out))
	}
}

func TestExecuteAPINoClient(t *testing.T) {
	router := NewRouter()
	if _, err := router.ExecuteAPI(context.Background(), "grafana", "annotate", nil, HTTPClients{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecuteAPIUnknownTool(t *testing.T) {
	router := NewRouter()
	if _, err := router.ExecuteAPI(context.Background(), "unknown", "x", nil, HTTPClients{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecuteAPINoClientBranches(t *testing.T) {
	router := NewRouter()
	if _, err := router.ExecuteAPI(context.Background(), "thanos", "query", nil, HTTPClients{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := router.ExecuteAPI(context.Background(), "tempo", "traceql", nil, HTTPClients{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := router.ExecuteAPI(context.Background(), "linear", "create", nil, HTTPClients{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := router.ExecuteAPI(context.Background(), "pagerduty", "create", nil, HTTPClients{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := router.ExecuteAPI(context.Background(), "vault", "login_k8s", nil, HTTPClients{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := router.ExecuteAPI(context.Background(), "boundary", "session_open", nil, HTTPClients{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := router.ExecuteAPI(context.Background(), "argocd", "status", nil, HTTPClients{}); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := router.ExecuteAPI(context.Background(), "aws", "cloudtrail_lookup_events", nil, HTTPClients{}); err == nil {
		t.Fatalf("expected error")
	}
}
