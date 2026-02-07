package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"carapulse/internal/policy"
)

func TestToolRouterServerMethodNotAllowed(t *testing.T) {
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerBadJSON(t *testing.T) {
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("{")))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerNotFound(t *testing.T) {
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerExecuteError(t *testing.T) {
	reqBody, _ := json.Marshal(ExecuteRequest{
		Tool:   "prometheus",
		Action: "query",
		Input:  map[string]any{"query": "up"},
		Context: ContextRef{
			TenantID:      "t",
			Environment:   "prod",
			ClusterID:     "c",
			Namespace:     "ns",
			AWSAccountID:  "a",
			Region:        "r",
			ArgoCDProject: "p",
			GrafanaOrgID:  "g",
		},
	})
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	srv.Policy = &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) {
		return policy.PolicyDecision{Decision: "allow"}, nil
	})}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerAuthMissing(t *testing.T) {
	reqBody, _ := json.Marshal(ExecuteRequest{Tool: "prometheus", Action: "query", Input: map[string]any{"query": "up"}})
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerPolicyDenied(t *testing.T) {
	reqBody, _ := json.Marshal(ExecuteRequest{
		Tool:   "kubectl",
		Action: "scale",
		Input:  map[string]any{"resource": "svc", "replicas": 1},
		Context: ContextRef{
			TenantID:      "t",
			Environment:   "prod",
			ClusterID:     "c",
			Namespace:     "ns",
			AWSAccountID:  "a",
			Region:        "r",
			ArgoCDProject: "p",
			GrafanaOrgID:  "g",
		},
	})
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	srv.Policy = &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) {
		return policy.PolicyDecision{Decision: "deny"}, nil
	})}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerPolicyMissingContext(t *testing.T) {
	reqBody, _ := json.Marshal(ExecuteRequest{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "svc", "replicas": 1}})
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	srv.Policy = &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) {
		return policy.PolicyDecision{Decision: "allow"}, nil
	})}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerOK(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer api.Close()

	reqBody, _ := json.Marshal(ExecuteRequest{
		Tool:   "prometheus",
		Action: "query",
		Input:  map[string]any{"query": "up"},
		Context: ContextRef{
			TenantID:      "t",
			Environment:   "prod",
			ClusterID:     "c",
			Namespace:     "ns",
			AWSAccountID:  "a",
			Region:        "r",
			ArgoCDProject: "p",
			GrafanaOrgID:  "g",
		},
	})
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{Prometheus: &APIClient{BaseURL: api.URL}})
	srv.Auth.Token = "token"
	srv.Policy = &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) {
		return policy.PolicyDecision{Decision: "allow"}, nil
	})}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Used != "api" {
		t.Fatalf("used: %s", resp.Used)
	}
	if string(resp.Output) != "ok" {
		t.Fatalf("output: %s", string(resp.Output))
	}
	if resp.ToolCallID == "" {
		t.Fatalf("missing tool call id")
	}
}

func TestToolRouterServerBreakGlassRequired(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/argocd"
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("path: %v", err)
	}
	defer func() { _ = os.Setenv("PATH", oldPath) }()

	reqBody, _ := json.Marshal(ExecuteRequest{
		Tool:   "argocd",
		Action: "rollback",
		Input:  map[string]any{"app": "app"},
		Context: ContextRef{
			TenantID:      "t",
			Environment:   "prod",
			ClusterID:     "c",
			Namespace:     "ns",
			AWSAccountID:  "a",
			Region:        "r",
			ArgoCDProject: "p",
			GrafanaOrgID:  "g",
		},
	})
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	srv.Policy = &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) {
		return policy.PolicyDecision{Decision: "allow"}, nil
	})}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerBreakGlassHeader(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/argocd"
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("path: %v", err)
	}
	defer func() { _ = os.Setenv("PATH", oldPath) }()

	reqBody, _ := json.Marshal(ExecuteRequest{
		Tool:   "argocd",
		Action: "rollback",
		Input:  map[string]any{"app": "app"},
		Context: ContextRef{
			TenantID:      "t",
			Environment:   "prod",
			ClusterID:     "c",
			Namespace:     "ns",
			AWSAccountID:  "a",
			Region:        "r",
			ArgoCDProject: "p",
			GrafanaOrgID:  "g",
		},
	})
	sandbox := &Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) { return []byte("ok"), nil }}
	srv := NewServer(NewRouter(), sandbox, HTTPClients{})
	srv.Auth.Token = "token"
	srv.Policy = &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) {
		return policy.PolicyDecision{Decision: "allow"}, nil
	})}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Break-Glass", "true")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerListToolsOK(t *testing.T) {
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodGet, "/v1/tools", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp ListToolsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Tools) == 0 {
		t.Fatalf("expected tools")
	}
}

func TestToolRouterServerListToolsMethodNotAllowed(t *testing.T) {
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodPost, "/v1/tools", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerResolveInline(t *testing.T) {
	body, _ := json.Marshal(ResolveResourceRequest{Artifact: ArtifactRef{Kind: "inline", Ref: "abc"}})
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodPost, "/v1/resources:resolve", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp ResolveResourceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(resp.Data) != "abc" {
		t.Fatalf("data: %s", string(resp.Data))
	}
}

func TestToolRouterServerResolveBadJSON(t *testing.T) {
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodPost, "/v1/resources:resolve", bytes.NewReader([]byte("{")))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerResolveUnsupported(t *testing.T) {
	body, _ := json.Marshal(ResolveResourceRequest{Artifact: ArtifactRef{Kind: "bad"}})
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodPost, "/v1/resources:resolve", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerResolveNotImplemented(t *testing.T) {
	oldResolve := resolveArtifact
	resolveArtifact = func(ref ArtifactRef) ([]byte, error) { return nil, ErrNotImplemented }
	defer func() { resolveArtifact = oldResolve }()

	body, _ := json.Marshal(ResolveResourceRequest{Artifact: ArtifactRef{Kind: "git_path", Ref: "repo/path"}})
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodPost, "/v1/resources:resolve", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerResolveInternalError(t *testing.T) {
	oldResolve := resolveArtifact
	resolveArtifact = func(ref ArtifactRef) ([]byte, error) { return nil, errors.New("boom") }
	defer func() { resolveArtifact = oldResolve }()

	body, _ := json.Marshal(ResolveResourceRequest{Artifact: ArtifactRef{Kind: "inline", Ref: "abc"}})
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodPost, "/v1/resources:resolve", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerResolveInvalid(t *testing.T) {
	oldResolve := resolveArtifact
	resolveArtifact = func(ref ArtifactRef) ([]byte, error) { return nil, ErrInvalidArtifact }
	defer func() { resolveArtifact = oldResolve }()

	body, _ := json.Marshal(ResolveResourceRequest{Artifact: ArtifactRef{Kind: "inline", Ref: "abc"}})
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodPost, "/v1/resources:resolve", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerResolveMethodNotAllowed(t *testing.T) {
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodGet, "/v1/resources:resolve", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerLogsMethodNotAllowed(t *testing.T) {
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodPost, "/v1/tools/logs", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerLogsMissingID(t *testing.T) {
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodGet, "/v1/tools/logs", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestToolRouterServerLogsRouterMissing(t *testing.T) {
	srv := NewServer(nil, NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodGet, "/v1/tools/logs?tool_call_id=tool_1", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

type noFlushWriter struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (w *noFlushWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *noFlushWriter) WriteHeader(code int) { w.code = code }
func (w *noFlushWriter) Write(p []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.body.Write(p)
}

func TestToolRouterServerLogsNoFlusher(t *testing.T) {
	srv := NewServer(NewRouter(), NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodGet, "/v1/tools/logs?tool_call_id=tool_1", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := &noFlushWriter{}
	srv.ServeHTTP(w, req)
	if w.code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.code)
	}
}

func TestToolRouterServerLogsOK(t *testing.T) {
	router := NewRouter()
	router.Logs.Append(LogLine{ToolCallID: "tool_1", Level: "info", Message: "hello", Timestamp: time.Now().UTC()})
	srv := NewServer(router, NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/v1/tools/logs?tool_call_id=tool_1", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		srv.ServeHTTP(w, req)
		close(done)
	}()
	waitForLogSub(t, router.Logs, "tool_1")
	router.Logs.Append(LogLine{ToolCallID: "tool_1", Level: "info", Message: "stream", Timestamp: time.Now().UTC()})
	cancel()
	<-done
	if !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("missing log")
	}
	if !strings.Contains(w.Body.String(), "stream") {
		t.Fatalf("missing stream log")
	}
}

func TestToolRouterServerLogsMarshalError(t *testing.T) {
	router := NewRouter()
	router.Logs.Append(LogLine{ToolCallID: "tool_1", Level: "info", Message: "hello", Timestamp: time.Now().UTC()})
	srv := NewServer(router, NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	oldMarshal := marshalLogJSON
	marshalLogJSON = func(v any) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { marshalLogJSON = oldMarshal })

	req := httptest.NewRequest(http.MethodGet, "/v1/tools/logs?tool_call_id=tool_1", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("unexpected log")
	}
}

func TestToolRouterServerLogsChannelClosed(t *testing.T) {
	router := NewRouter()
	srv := NewServer(router, NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	req := httptest.NewRequest(http.MethodGet, "/v1/tools/logs?tool_call_id=tool_1", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		srv.ServeHTTP(w, req)
		close(done)
	}()
	waitForLogSub(t, router.Logs, "tool_1")

	router.Logs.mu.Lock()
	subs := router.Logs.subs["tool_1"]
	for id, ch := range subs {
		close(ch)
		delete(subs, id)
	}
	if len(subs) == 0 {
		delete(router.Logs.subs, "tool_1")
	}
	router.Logs.mu.Unlock()

	<-done
}

func TestToolRouterServerLogsStreamMarshalError(t *testing.T) {
	router := NewRouter()
	srv := NewServer(router, NewSandbox(), HTTPClients{})
	srv.Auth.Token = "token"
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/v1/tools/logs?tool_call_id=tool_1", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	done := make(chan struct{})

	oldMarshal := marshalLogJSON
	marshalLogJSON = func(v any) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { marshalLogJSON = oldMarshal })

	go func() {
		srv.ServeHTTP(w, req)
		close(done)
	}()
	waitForLogSub(t, router.Logs, "tool_1")
	router.Logs.Append(LogLine{ToolCallID: "tool_1", Level: "info", Message: "stream", Timestamp: time.Now().UTC()})
	cancel()
	<-done
}

func waitForLogSub(t *testing.T, hub *LogHub, toolCallID string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		hub.mu.Lock()
		_, ok := hub.subs[toolCallID]
		hub.mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for subscriber")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
