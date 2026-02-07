package chatops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"carapulse/internal/web"
)

func TestGatewayClientCreatePlan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/plans" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.Write([]byte(`{"plan_id":"plan_1"}`))
	}))
	defer srv.Close()

	client := &HTTPGatewayClient{BaseURL: srv.URL}
	id, err := client.CreatePlan(context.Background(), web.PlanCreateRequest{Summary: "s"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "plan_1" {
		t.Fatalf("id: %s", id)
	}
}

func TestGatewayClientCreatePlanNested(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"plan":{"plan_id":"plan_2"}}`))
	}))
	defer srv.Close()

	client := &HTTPGatewayClient{BaseURL: srv.URL}
	id, err := client.CreatePlan(context.Background(), web.PlanCreateRequest{Summary: "s"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "plan_2" {
		t.Fatalf("id: %s", id)
	}
}

func TestGatewayClientCreatePlanMissingID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := &HTTPGatewayClient{BaseURL: srv.URL}
	if _, err := client.CreatePlan(context.Background(), web.PlanCreateRequest{Summary: "s"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientCreatePlanDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{"))
	}))
	defer srv.Close()

	client := &HTTPGatewayClient{BaseURL: srv.URL}
	if _, err := client.CreatePlan(context.Background(), web.PlanCreateRequest{Summary: "s"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientCreateApproval(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/approvals" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := &HTTPGatewayClient{BaseURL: srv.URL}
	if err := client.CreateApproval(context.Background(), "plan", "approved", ""); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestGatewayClientGetExecution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/executions/exec" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.Write([]byte(`{"execution_id":"exec"}`))
	}))
	defer srv.Close()

	client := &HTTPGatewayClient{BaseURL: srv.URL}
	resp, err := client.GetExecution(context.Background(), "exec")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(resp) == "" {
		t.Fatalf("empty")
	}
}

func TestGatewayClientListAudit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audit/events" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if r.URL.RawQuery == "" {
			t.Fatalf("expected query")
		}
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	client := &HTTPGatewayClient{BaseURL: srv.URL}
	out, err := client.ListAudit(context.Background(), url.Values{"plan_id": []string{"p"}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != "[]" {
		t.Fatalf("out: %s", string(out))
	}
}

func TestGatewayClientStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer srv.Close()

	client := &HTTPGatewayClient{BaseURL: srv.URL}
	if _, err := client.GetExecution(context.Background(), "exec"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientBadBaseURL(t *testing.T) {
	client := &HTTPGatewayClient{BaseURL: "http://%"}
	if _, err := client.GetExecution(context.Background(), "exec"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientMarshalError(t *testing.T) {
	client := &HTTPGatewayClient{BaseURL: "http://example.com"}
	if err := client.doJSON(context.Background(), http.MethodPost, "/v1/plans", make(chan int), &struct{}{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientRequestError(t *testing.T) {
	client := &HTTPGatewayClient{BaseURL: "http://example.com"}
	client.Client = &http.Client{Transport: errRoundTripper{}}
	if _, err := client.GetExecution(context.Background(), "exec"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGatewayClientDoJSONNoOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := &HTTPGatewayClient{BaseURL: srv.URL}
	if err := client.doJSON(context.Background(), http.MethodGet, "/v1/audit/events", nil, nil); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestGatewayClientTokenNoBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("auth: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "" {
			t.Fatalf("content-type: %s", r.Header.Get("Content-Type"))
		}
		w.Write([]byte(`ok`))
	}))
	defer srv.Close()

	client := &HTTPGatewayClient{BaseURL: srv.URL, Token: "token"}
	if _, err := client.doRequest(context.Background(), http.MethodGet, "/v1/audit/events", nil); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestGatewayClientNewRequestError(t *testing.T) {
	client := &HTTPGatewayClient{BaseURL: "http://example.com"}
	if _, err := client.doRequest(context.Background(), " ", "/v1/plans", nil); err == nil {
		t.Fatalf("expected error")
	}
}

type errRoundTripper struct{}

func (e errRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errClient
}
