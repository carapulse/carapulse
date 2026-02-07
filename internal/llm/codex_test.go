package llm

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCodexCompleteMissingToken(t *testing.T) {
	client := &CodexClient{Model: "gpt"}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCodexCompleteMissingModel(t *testing.T) {
	client := &CodexClient{AccessToken: "token"}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCodexCompleteMarshalError(t *testing.T) {
	old := marshalJSON
	marshalJSON = func(v any) ([]byte, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() { marshalJSON = old })

	client := &CodexClient{AccessToken: "token", Model: "gpt"}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCodexCompleteStatusError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad"))
	}))
	defer ts.Close()

	client := &CodexClient{APIBase: ts.URL, AccessToken: "token", Model: "gpt", HTTPClient: ts.Client()}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCodexCompleteBadJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("nope"))
	}))
	defer ts.Close()

	client := &CodexClient{APIBase: ts.URL, AccessToken: "token", Model: "gpt", HTTPClient: ts.Client()}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCodexCompleteEmptyChoices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer ts.Close()

	client := &CodexClient{APIBase: ts.URL, AccessToken: "token", Model: "gpt", HTTPClient: ts.Client()}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCodexCompleteEmptyContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`))
	}))
	defer ts.Close()

	client := &CodexClient{APIBase: ts.URL, AccessToken: "token", Model: "gpt", HTTPClient: ts.Client()}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCodexCompleteOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("auth: %s", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload["model"] != "gpt" {
			t.Fatalf("model: %#v", payload["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer ts.Close()

	client := &CodexClient{APIBase: ts.URL, AccessToken: "token", Model: "gpt", HTTPClient: ts.Client()}
	out, err := client.Complete("prompt", 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("output: %s", out)
	}
}

func TestCodexCompleteDefaultBase(t *testing.T) {
	client := &CodexClient{
		AccessToken: "token",
		Model:       "gpt",
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Host != "api.openai.com" {
				t.Fatalf("host: %s", r.URL.Host)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	out, err := client.Complete("prompt", 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("output: %s", out)
	}
}

func TestCodexCompleteHTTPClientNil(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer ts.Close()

	client := &CodexClient{APIBase: ts.URL, AccessToken: "token", Model: "gpt"}
	out, err := client.Complete("prompt", 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("output: %s", out)
	}
}

func TestCodexCompleteRequestError(t *testing.T) {
	client := &CodexClient{APIBase: "http://[::1]:namedport", AccessToken: "token", Model: "gpt"}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCodexCompleteDoError(t *testing.T) {
	client := &CodexClient{
		APIBase:     "http://example.com",
		AccessToken: "token",
		Model:       "gpt",
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})},
	}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}
