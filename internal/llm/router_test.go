package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	input := "token=abc123 secret=xyz"
	out := Redact(input, []string{"token=\\w+", "["})
	if out == input {
		t.Fatalf("expected redaction")
	}
	if out == "" {
		t.Fatalf("unexpected empty output")
	}
}

func TestRedactBadRegex(t *testing.T) {
	input := "value=1"
	out := Redact(input, []string{"[", "("})
	if out != input {
		t.Fatalf("unexpected change")
	}
}

func TestPlanMissingIntent(t *testing.T) {
	router := &Router{Provider: "openai"}
	if _, err := router.Plan("  ", nil, nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPlanUnknownProvider(t *testing.T) {
	router := &Router{Provider: "unknown", APIKey: "k"}
	if _, err := router.Plan("intent", nil, nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPlanPromptError(t *testing.T) {
	router := &Router{Provider: "openai", APIKey: "k", Model: "m"}
	if _, err := router.Plan("intent", make(chan int), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPlanEvidenceError(t *testing.T) {
	router := &Router{Provider: "openai", APIKey: "k", Model: "m"}
	if _, err := router.Plan("intent", nil, make(chan int)); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPlanOpenAIEnvKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer envkey" {
			t.Fatalf("auth: %s", got)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/chat/completions") {
			t.Fatalf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer ts.Close()

	old := os.Getenv("OPENAI_API_KEY")
	if err := os.Setenv("OPENAI_API_KEY", "envkey"); err != nil {
		t.Fatalf("env: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("OPENAI_API_KEY", old) })

	router := &Router{
		Provider:   "openai",
		APIBase:    ts.URL,
		Model:      "gpt",
		HTTPClient: ts.Client(),
	}
	out, err := router.Plan("intent", map[string]any{"env": "prod"}, []string{"e1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("output: %s", out)
	}
}

func TestPlanAnthropicOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "key" {
			t.Fatalf("key: %s", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Fatalf("missing version")
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/messages") {
			t.Fatalf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"plan"}]}`))
	}))
	defer ts.Close()

	router := &Router{
		Provider:   "anthropic",
		APIBase:    ts.URL,
		Model:      "claude",
		APIKey:     "key",
		HTTPClient: ts.Client(),
		MaxTokens:  10,
	}
	out, err := router.Plan("intent", nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(out) != "plan" {
		t.Fatalf("output: %s", out)
	}
}

func TestPlanAnthropicEnvKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "envkey" {
			t.Fatalf("key: %s", got)
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer ts.Close()

	old := os.Getenv("ANTHROPIC_API_KEY")
	if err := os.Setenv("ANTHROPIC_API_KEY", "envkey"); err != nil {
		t.Fatalf("env: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("ANTHROPIC_API_KEY", old) })

	router := &Router{
		Provider:   "anthropic",
		APIBase:    ts.URL,
		Model:      "claude",
		HTTPClient: ts.Client(),
	}
	out, err := router.Plan("intent", nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("output: %s", out)
	}
}

func TestPlanCodexProfile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Fatalf("auth: %s", got)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer ts.Close()

	path := filepath.Join(t.TempDir(), "auth.json")
	profiles := AuthProfiles{Profiles: []AuthProfile{{ID: "p1", Provider: codexProvider, AccessToken: "tok"}}}
	if err := SaveAuthProfiles(path, profiles); err != nil {
		t.Fatalf("save: %v", err)
	}

	router := &Router{
		Provider:    codexProvider,
		APIBase:     ts.URL,
		Model:       "gpt",
		HTTPClient:  ts.Client(),
		AuthPath:    path,
		AuthProfile: "p1",
	}
	out, err := router.Plan("intent", nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("output: %s", out)
	}
}

func TestPlanRedactsPrompt(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req openAIRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		prompt := req.Messages[0].Content
		if strings.Contains(prompt, "secret=abc") {
			t.Fatalf("prompt not redacted: %s", prompt)
		}
		if !strings.Contains(prompt, "[REDACTED]") {
			t.Fatalf("missing redaction: %s", prompt)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer ts.Close()

	router := &Router{
		Provider:       "openai",
		APIBase:        ts.URL,
		Model:          "gpt",
		APIKey:         "k",
		HTTPClient:     ts.Client(),
		RedactPatterns: []string{"secret=\\w+"},
	}
	if _, err := router.Plan("secret=abc", nil, nil); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestPlanCodexResolveError(t *testing.T) {
	t.Setenv("OPENAI_ACCESS_TOKEN", "")
	t.Setenv("CARAPULSE_AUTH_PROFILE", "")
	t.Setenv("CARAPULSE_AUTH_PATH", "")
	t.Setenv("CARAPULSE_STATE_DIR", t.TempDir())
	router := &Router{Provider: codexProvider}
	if _, err := router.Plan("intent", nil, nil); err == nil {
		t.Fatalf("expected error")
	}
}
