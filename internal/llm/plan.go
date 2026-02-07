package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

func (r *Router) Plan(intent string, context any, evidence any) (string, error) {
	intent = strings.TrimSpace(intent)
	if intent == "" {
		return "", errors.New("intent required")
	}
	prompt, err := buildPrompt(intent, context, evidence)
	if err != nil {
		return "", err
	}
	if len(r.RedactPatterns) > 0 {
		prompt = Redact(prompt, r.RedactPatterns)
	}
	maxTokens := r.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 512
	}
	provider := strings.ToLower(strings.TrimSpace(r.Provider))
	switch provider {
	case "openai":
		key := r.APIKey
		if key == "" {
			key = os.Getenv("OPENAI_API_KEY")
		}
		client := &OpenAIClient{
			APIBase:    r.APIBase,
			APIKey:     key,
			Model:      r.Model,
			HTTPClient: r.HTTPClient,
		}
		return client.Complete(prompt, maxTokens)
	case "anthropic":
		key := r.APIKey
		if key == "" {
			key = os.Getenv("ANTHROPIC_API_KEY")
		}
		client := &AnthropicClient{
			APIBase:    r.APIBase,
			APIKey:     key,
			Model:      r.Model,
			HTTPClient: r.HTTPClient,
		}
		return client.Complete(prompt, maxTokens)
	case codexProvider:
		token, err := r.resolveCodexToken()
		if err != nil {
			return "", err
		}
		client := &CodexClient{
			APIBase:     r.APIBase,
			AccessToken: token,
			Model:       r.Model,
			HTTPClient:  r.HTTPClient,
		}
		return client.Complete(prompt, maxTokens)
	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}
}

func buildPrompt(intent string, context any, evidence any) (string, error) {
	ctxJSON, err := json.Marshal(context)
	if err != nil {
		return "", err
	}
	evJSON, err := json.Marshal(evidence)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"Intent:\n%s\nContext:\n%s\nEvidence:\n%s\nReturn JSON only with shape:\n{\"summary\":string,\"risk_level\":\"low|medium|high\",\"steps\":[{\"action\":string,\"tool\":string,\"input\":object,\"preconditions\":array,\"rollback\":object}]}",
		intent,
		string(ctxJSON),
		string(evJSON),
	), nil
}
