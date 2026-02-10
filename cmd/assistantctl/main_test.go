package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"carapulse/internal/web"
)

func TestRunMissingCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"nope"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunHelp(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"--help"}, &buf); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "Usage: assistantctl") {
		t.Fatalf("usage: %s", buf.String())
	}
}

func TestRunVersion(t *testing.T) {
	oldVersion := version
	oldCommit := commit
	version = "1.2.3"
	commit = "abc"
	defer func() {
		version = oldVersion
		commit = oldCommit
	}()
	var buf bytes.Buffer
	if err := run([]string{"--version"}, &buf); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "1.2.3 (abc)" {
		t.Fatalf("version: %s", got)
	}
}

func TestRunPlanUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "nope"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanMissingSubcommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanCreateBadFlag(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "create", "-badflag"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanCreateMissingSummary(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "create", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanCreateBadJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "create", "-summary", "s", "-context", "{", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanCreateMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "create", "-summary", "s"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanCreateMissingPlanID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	if err := run([]string{"plan", "create", "-summary", "s", "-gateway", ts.URL}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanCreateOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/plans" || r.Method != http.MethodPost {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload["summary"] != "summary" {
			t.Fatalf("summary: %#v", payload["summary"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"plan_id":"plan_1"}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{
		"plan", "create",
		"-summary", "summary",
		"-intent", "intent",
		"-context", `{"tenant_id":"t"}`,
		"-constraints", `{"max":1}`,
		"-gateway", ts.URL,
		"-token", "tok",
	}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "plan_1" {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunPlanCreateNestedPlanID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"plan":{"plan_id":"plan_2"}}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"plan", "create", "-summary", "summary", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "plan_2" {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunPlanCreateDefaultIntent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload["intent"] != "summary" {
			t.Fatalf("intent: %#v", payload["intent"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"plan_id":"plan_2"}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"plan", "create", "-summary", "summary", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "plan_2" {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunPlanCreateBadConstraints(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "create", "-summary", "s", "-constraints", "{", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanApproveMissingPlanID(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "approve", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanApproveBadFlag(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "approve", "-badflag"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanApproveMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "approve", "-plan-id", "plan_1"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanApproveOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/approvals" || r.Method != http.MethodPost {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload["plan_id"] != "plan_1" || payload["status"] != "approved" {
			t.Fatalf("payload: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"approved"}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"plan", "approve", "-plan-id", "plan_1", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "ok" {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunPlanApproveGatewayError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	if err := run([]string{"plan", "approve", "-plan-id", "plan_1", "-gateway", ts.URL}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunExecUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"exec", "nope"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunExecMissingSubcommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"exec"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunExecLogsMissingID(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"exec", "logs", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunExecLogsBadFlag(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"exec", "logs", "-badflag"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunExecLogsMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"exec", "logs", "-execution-id", "exec_1"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunExecLogsOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/executions/exec_1/logs" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("level"); got != "info" {
			t.Fatalf("level: %s", got)
		}
		if got := r.URL.Query().Get("tool_call_id"); got != "tool_1" {
			t.Fatalf("tool_call_id: %s", got)
		}
		_, _ = w.Write([]byte("logline"))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"exec", "logs", "-execution-id", "exec_1", "-level", "info", "-tool-call-id", "tool_1", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if buf.String() != "logline" {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunExecLogsGatewayError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	if err := run([]string{"exec", "logs", "-execution-id", "exec_1", "-gateway", ts.URL}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunContextUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"context", "nope"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunContextMissingSubcommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"context"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunContextRefreshMissingService(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"context", "refresh", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunContextRefreshMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"context", "refresh", "-service", "svc"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunContextRefreshBadFlag(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"context", "refresh", "-badflag"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunContextRefreshOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/context/refresh" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload["service"] != "svc" {
			t.Fatalf("payload: %#v", payload)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"context", "refresh", "-service", "svc", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "ok" {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunContextRefreshGatewayError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	if err := run([]string{"context", "refresh", "-service", "svc", "-gateway", ts.URL}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPolicyUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"policy", "nope"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPolicyMissingSubcommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"policy"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPolicyTestMissingOPA(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"policy", "test", "-input", "{}"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPolicyTestMissingInput(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"policy", "test", "-opa-url", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPolicyTestBadJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"policy", "test", "-opa-url", "http://example", "-input", "{"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPolicyTestBadFlag(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"policy", "test", "-badflag"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPolicyTestOPAError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	if err := run([]string{"policy", "test", "-opa-url", ts.URL, "-input", "{}"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPolicyTestOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/data/") {
			t.Fatalf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"result":{"decision":"allow","constraints":{"k":"v"},"ttl":0}}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"policy", "test", "-opa-url", ts.URL, "-input", `{"action":"x"}`}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "\"decision\":\"allow\"") {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestParseJSONInputEmpty(t *testing.T) {
	var out map[string]any
	if err := parseJSONInput("", &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestParseJSONInputFile(t *testing.T) {
	file := t.TempDir() + "/input.json"
	if err := os.WriteFile(file, []byte(`{"key":"value"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var out map[string]any
	if err := parseJSONInput("@"+file, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if out["key"] != "value" {
		t.Fatalf("out: %#v", out)
	}
}

func TestReadInputEmptyPath(t *testing.T) {
	if _, err := readInput("@"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseJSONInputReadError(t *testing.T) {
	oldRead := readFile
	readFile = func(path string) ([]byte, error) {
		return nil, errors.New("boom")
	}
	defer func() { readFile = oldRead }()

	var out map[string]any
	if err := parseJSONInput("@/nope", &out); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientDoRequestBadURL(t *testing.T) {
	client := &gatewayClient{BaseURL: "://bad"}
	if _, err := client.doRequest(context.Background(), http.MethodGet, "/v1/plans", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientDoRequestBadJSON(t *testing.T) {
	client := &gatewayClient{BaseURL: "http://example.com"}
	if _, err := client.doRequest(context.Background(), http.MethodPost, "/v1/plans", map[string]any{"bad": math.Inf(1)}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientDoRequestStatusError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("nope"))
	}))
	defer ts.Close()

	client := &gatewayClient{BaseURL: ts.URL}
	if _, err := client.doRequest(context.Background(), http.MethodGet, "/v1/plans", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientDoRequestBadMethod(t *testing.T) {
	client := &gatewayClient{BaseURL: "http://example.com"}
	if _, err := client.doRequest(context.Background(), "bad method", "/v1/plans", nil); err == nil {
		t.Fatalf("expected error")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestGatewayClientDoRequestTransportError(t *testing.T) {
	client := &gatewayClient{
		BaseURL: "http://example.com",
		Client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})},
	}
	if _, err := client.doRequest(context.Background(), http.MethodGet, "/v1/plans", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientDoJSONBadResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("nope"))
	}))
	defer ts.Close()

	client := &gatewayClient{BaseURL: ts.URL}
	if _, err := client.CreatePlan(context.Background(), web.PlanCreateRequest{Summary: "s", Trigger: "manual"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientDoJSONRequestError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := &gatewayClient{BaseURL: ts.URL}
	if err := client.doJSON(context.Background(), http.MethodGet, "/v1/plans", nil, nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientAuthorizationHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Fatalf("auth: %s", got)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	client := &gatewayClient{BaseURL: ts.URL, Token: "tok"}
	if err := client.doJSON(context.Background(), http.MethodPost, "/v1/approvals", map[string]any{}, nil); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestRunScheduleMissingSubcommand(t *testing.T) {
	if err := run([]string{"schedule"}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunScheduleCreateOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/schedules" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"schedule_id":"sched_1"}`))
	}))
	defer ts.Close()
	var buf bytes.Buffer
	args := []string{
		"schedule", "create",
		"--summary", "s",
		"--cron", "*/5 * * * *",
		"--intent", "i",
		"--context", `{"tenant_id":"t"}`,
		"--gateway", ts.URL,
	}
	if err := run(args, &buf); err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "sched_1" {
		t.Fatalf("out: %s", buf.String())
	}
}

func TestRunScheduleListOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/schedules" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"schedule_id":"s1"}]`))
	}))
	defer ts.Close()
	var buf bytes.Buffer
	if err := run([]string{"schedule", "list", "--gateway", ts.URL}, &buf); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "s1") {
		t.Fatalf("out: %s", buf.String())
	}
}

func TestGatewayClientCreateScheduleMissingID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()
	client := &gatewayClient{BaseURL: ts.URL}
	if _, err := client.CreateSchedule(context.Background(), web.ScheduleCreateRequest{Summary: "s", Cron: "* * * * *"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientListSchedules(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()
	client := &gatewayClient{BaseURL: ts.URL}
	if _, err := client.ListSchedules(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
}

// --- plan get ---

func TestRunPlanGetMissingPlanID(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "get", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanGetMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "get", "-plan-id", "plan_1"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanGetBadFlag(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "get", "-badflag"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanGetOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/plans/plan_1" || r.Method != http.MethodGet {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		_, _ = w.Write([]byte(`{"plan_id":"plan_1","summary":"test"}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"plan", "get", "-plan-id", "plan_1", "-gateway", ts.URL, "-token", "tok"}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "plan_1") {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunPlanGetGatewayError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	if err := run([]string{"plan", "get", "-plan-id", "plan_1", "-gateway", ts.URL}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

// --- plan execute ---

func TestRunPlanExecuteMissingPlanID(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "execute", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanExecuteMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "execute", "-plan-id", "plan_1"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanExecuteBadFlag(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"plan", "execute", "-badflag"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanExecuteOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/plans/plan_1:execute" || r.Method != http.MethodPost {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload["approval_token"] != "tok123" {
			t.Fatalf("payload: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"execution_id":"exec_1"}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"plan", "execute", "-plan-id", "plan_1", "-approval-token", "tok123", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "exec_1" {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunPlanExecuteMissingExecID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	if err := run([]string{"plan", "execute", "-plan-id", "plan_1", "-gateway", ts.URL}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

// --- workflow list ---

func TestRunWorkflowListOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workflows" || r.Method != http.MethodGet {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		if got := r.Header.Get("X-Tenant-Id"); got != "t1" {
			t.Fatalf("tenant: %s", got)
		}
		_, _ = w.Write([]byte(`{"workflows":[{"name":"gitops_deploy"}]}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"workflow", "list", "-gateway", ts.URL, "-tenant-id", "t1"}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "gitops_deploy") {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunWorkflowListMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"workflow", "list"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

// --- workflow start ---

func TestRunWorkflowStartOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workflows/gitops_deploy/start" || r.Method != http.MethodPost {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		_, _ = w.Write([]byte(`{"plan_id":"plan_1","status":"pending"}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{
		"workflow", "start",
		"-name", "gitops_deploy",
		"-context", `{"tenant_id":"t"}`,
		"-input", `{"argocd_app":"myapp"}`,
		"-gateway", ts.URL,
	}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "plan_1") {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunWorkflowStartMissingName(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"workflow", "start", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunWorkflowStartMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"workflow", "start", "-name", "gitops_deploy"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunWorkflowStartBadContext(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"workflow", "start", "-name", "n", "-context", "{", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunWorkflowStartBadInput(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"workflow", "start", "-name", "n", "-input", "{", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunWorkflowStartBadConstraints(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"workflow", "start", "-name", "n", "-constraints", "{", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

// --- session create ---

func TestRunSessionMissingSubcommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"session"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunSessionUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"session", "nope"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunSessionCreateOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sessions" || r.Method != http.MethodPost {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload["name"] != "mysession" || payload["tenant_id"] != "t1" {
			t.Fatalf("payload: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session_id":"sess_1"}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"session", "create", "-name", "mysession", "-tenant-id", "t1", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "sess_1" {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunSessionCreateMissingName(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"session", "create", "-tenant-id", "t", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunSessionCreateMissingTenantID(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"session", "create", "-name", "n", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunSessionCreateMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"session", "create", "-name", "n", "-tenant-id", "t"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunSessionCreateMissingSessionID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	if err := run([]string{"session", "create", "-name", "n", "-tenant-id", "t", "-gateway", ts.URL}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

// --- session list ---

func TestRunSessionListOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sessions" || r.Method != http.MethodGet {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		_, _ = w.Write([]byte(`[{"session_id":"sess_1"}]`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"session", "list", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "sess_1") {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunSessionListMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"session", "list"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

// --- playbook list ---

func TestRunPlaybookMissingSubcommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"playbook"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlaybookUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"playbook", "nope"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlaybookListOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/playbooks" || r.Method != http.MethodGet {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		_, _ = w.Write([]byte(`[{"id":"pb_1"}]`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"playbook", "list", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "pb_1") {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunPlaybookListMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"playbook", "list"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

// --- playbook get ---

func TestRunPlaybookGetOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/playbooks/pb_1" || r.Method != http.MethodGet {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		_, _ = w.Write([]byte(`{"id":"pb_1","name":"deploy"}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"playbook", "get", "-playbook-id", "pb_1", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "pb_1") {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunPlaybookGetMissingID(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"playbook", "get", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlaybookGetMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"playbook", "get", "-playbook-id", "pb_1"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlaybookGetGatewayError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	if err := run([]string{"playbook", "get", "-playbook-id", "pb_1", "-gateway", ts.URL}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

// --- runbook list ---

func TestRunRunbookMissingSubcommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"runbook"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunRunbookUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"runbook", "nope"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunRunbookListOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/runbooks" || r.Method != http.MethodGet {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		_, _ = w.Write([]byte(`[{"id":"rb_1"}]`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"runbook", "list", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "rb_1") {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunRunbookListMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"runbook", "list"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

// --- runbook get ---

func TestRunRunbookGetOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/runbooks/rb_1" || r.Method != http.MethodGet {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		_, _ = w.Write([]byte(`{"id":"rb_1","name":"incident"}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"runbook", "get", "-runbook-id", "rb_1", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "rb_1") {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunRunbookGetMissingID(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"runbook", "get", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunRunbookGetMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"runbook", "get", "-runbook-id", "rb_1"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunRunbookGetGatewayError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	if err := run([]string{"runbook", "get", "-runbook-id", "rb_1", "-gateway", ts.URL}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

// --- audit list ---

func TestRunAuditMissingSubcommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"audit"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunAuditUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"audit", "nope"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunAuditListOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audit/events" || r.Method != http.MethodGet {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		_, _ = w.Write([]byte(`[{"event":"plan.create"}]`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"audit", "list", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "plan.create") {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunAuditListWithTenantFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("tenant_id"); got != "t1" {
			t.Fatalf("tenant_id: %s", got)
		}
		_, _ = w.Write([]byte(`[{"event":"plan.create"}]`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"audit", "list", "-tenant-id", "t1", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestRunAuditListMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"audit", "list"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

// --- context snapshot ---

func TestRunContextSnapshotOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/context/snapshots" || r.Method != http.MethodGet {
			t.Fatalf("path: %s method: %s", r.URL.Path, r.Method)
		}
		if got := r.Header.Get("X-Tenant-Id"); got != "t1" {
			t.Fatalf("tenant: %s", got)
		}
		_, _ = w.Write([]byte(`[{"snapshot_id":"snap_1"}]`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := run([]string{"context", "snapshot", "-tenant-id", "t1", "-gateway", ts.URL}, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "snap_1") {
		t.Fatalf("output: %s", buf.String())
	}
}

func TestRunContextSnapshotMissingTenantID(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"context", "snapshot", "-gateway", "http://example"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunContextSnapshotMissingGateway(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"context", "snapshot", "-tenant-id", "t1"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunContextSnapshotGatewayError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	if err := run([]string{"context", "snapshot", "-tenant-id", "t1", "-gateway", ts.URL}, &buf); err == nil {
		t.Fatalf("expected error")
	}
}

// --- gateway client new methods ---

func TestGatewayClientGetPlan(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/plans/plan_1" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"plan_id":"plan_1"}`))
	}))
	defer ts.Close()
	client := &gatewayClient{BaseURL: ts.URL}
	data, err := client.GetPlan(context.Background(), "plan_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(string(data), "plan_1") {
		t.Fatalf("data: %s", data)
	}
}

func TestGatewayClientExecutePlanMissingID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()
	client := &gatewayClient{BaseURL: ts.URL}
	if _, err := client.ExecutePlan(context.Background(), "plan_1", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientCreateSessionMissingID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()
	client := &gatewayClient{BaseURL: ts.URL}
	if _, err := client.CreateSession(context.Background(), web.SessionRequest{Name: "n", TenantID: "t"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientDoRequestWithTenantBadURL(t *testing.T) {
	client := &gatewayClient{BaseURL: "://bad"}
	if _, err := client.doRequestWithTenant(context.Background(), http.MethodGet, "/v1/workflows", nil, "t"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestMainFatalOnError(t *testing.T) {
	oldFatal := fatalf
	called := false
	fatalf = func(format string, args ...any) { called = true }
	defer func() { fatalf = oldFatal }()

	oldArgs := os.Args
	os.Args = []string{"assistantctl"}
	defer func() { os.Args = oldArgs }()

	main()
	if !called {
		t.Fatalf("expected fatal")
	}
}
