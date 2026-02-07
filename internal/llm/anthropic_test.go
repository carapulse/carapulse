package llm

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicCompleteMissingKey(t *testing.T) {
	client := &AnthropicClient{Model: "claude"}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAnthropicCompleteMissingModel(t *testing.T) {
	client := &AnthropicClient{APIKey: "key"}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAnthropicCompleteMarshalError(t *testing.T) {
	old := marshalJSON
	marshalJSON = func(v any) ([]byte, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() { marshalJSON = old })

	client := &AnthropicClient{APIKey: "key", Model: "claude"}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAnthropicCompleteStatusError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad"))
	}))
	defer ts.Close()

	client := &AnthropicClient{APIBase: ts.URL, APIKey: "key", Model: "claude", HTTPClient: ts.Client()}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAnthropicCompleteBadJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("nope"))
	}))
	defer ts.Close()

	client := &AnthropicClient{APIBase: ts.URL, APIKey: "key", Model: "claude", HTTPClient: ts.Client()}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAnthropicCompleteEmptyContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":""}]}`))
	}))
	defer ts.Close()

	client := &AnthropicClient{APIBase: ts.URL, APIKey: "key", Model: "claude", HTTPClient: ts.Client()}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAnthropicCompleteEmptyBlocks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[]}`))
	}))
	defer ts.Close()

	client := &AnthropicClient{APIBase: ts.URL, APIKey: "key", Model: "claude", HTTPClient: ts.Client()}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAnthropicCompleteOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "key" {
			t.Fatalf("key: %s", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Fatalf("missing version")
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer ts.Close()

	client := &AnthropicClient{APIBase: ts.URL, APIKey: "key", Model: "claude", HTTPClient: ts.Client()}
	out, err := client.Complete("prompt", 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("output: %s", out)
	}
}

func TestAnthropicCompleteDefaultBase(t *testing.T) {
	client := &AnthropicClient{
		APIKey: "key",
		Model:  "claude",
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Host != "api.anthropic.com" {
				t.Fatalf("host: %s", r.URL.Host)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"ok"}]}`)),
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

func TestAnthropicCompleteHTTPClientNil(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer ts.Close()

	client := &AnthropicClient{APIBase: ts.URL, APIKey: "key", Model: "claude"}
	out, err := client.Complete("prompt", 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("output: %s", out)
	}
}

func TestAnthropicCompleteRequestError(t *testing.T) {
	client := &AnthropicClient{APIBase: "http://[::1]:namedport", APIKey: "key", Model: "claude"}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAnthropicCompleteDoError(t *testing.T) {
	client := &AnthropicClient{
		APIBase: "http://example.com",
		APIKey:  "key",
		Model:   "claude",
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})},
	}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}
