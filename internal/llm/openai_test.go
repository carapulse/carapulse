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

func TestOpenAICompleteMissingKey(t *testing.T) {
	client := &OpenAIClient{Model: "gpt"}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestOpenAICompleteMissingModel(t *testing.T) {
	client := &OpenAIClient{APIKey: "key"}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestOpenAICompleteMarshalError(t *testing.T) {
	old := marshalJSON
	marshalJSON = func(v any) ([]byte, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() { marshalJSON = old })

	client := &OpenAIClient{APIKey: "key", Model: "gpt"}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestOpenAICompleteStatusError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad"))
	}))
	defer ts.Close()

	client := &OpenAIClient{APIBase: ts.URL, APIKey: "key", Model: "gpt", HTTPClient: ts.Client()}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestOpenAICompleteBadJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("nope"))
	}))
	defer ts.Close()

	client := &OpenAIClient{APIBase: ts.URL, APIKey: "key", Model: "gpt", HTTPClient: ts.Client()}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestOpenAICompleteEmptyChoices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer ts.Close()

	client := &OpenAIClient{APIBase: ts.URL, APIKey: "key", Model: "gpt", HTTPClient: ts.Client()}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestOpenAICompleteEmptyContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`))
	}))
	defer ts.Close()

	client := &OpenAIClient{APIBase: ts.URL, APIKey: "key", Model: "gpt", HTTPClient: ts.Client()}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestOpenAICompleteOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer key" {
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

	client := &OpenAIClient{APIBase: ts.URL, APIKey: "key", Model: "gpt", HTTPClient: ts.Client()}
	out, err := client.Complete("prompt", 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("output: %s", out)
	}
}

func TestOpenAICompleteDefaultBase(t *testing.T) {
	client := &OpenAIClient{
		APIKey: "key",
		Model:  "gpt",
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

func TestOpenAICompleteHTTPClientNil(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer ts.Close()

	client := &OpenAIClient{APIBase: ts.URL, APIKey: "key", Model: "gpt"}
	out, err := client.Complete("prompt", 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("output: %s", out)
	}
}

func TestOpenAICompleteRequestError(t *testing.T) {
	client := &OpenAIClient{APIBase: "http://[::1]:namedport", APIKey: "key", Model: "gpt"}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestOpenAICompleteDoError(t *testing.T) {
	client := &OpenAIClient{
		APIBase: "http://example.com",
		APIKey:  "key",
		Model:   "gpt",
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})},
	}
	if _, err := client.Complete("prompt", 10); err == nil {
		t.Fatalf("expected error")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
