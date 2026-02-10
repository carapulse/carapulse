package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultAnthropicBase = "https://api.anthropic.com"
const anthropicVersion = "2023-06-01"

type AnthropicClient struct {
	APIBase    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (c *AnthropicClient) Complete(prompt string, maxTokens int) (string, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return "", errors.New("anthropic api key required")
	}
	if strings.TrimSpace(c.Model) == "" {
		return "", errors.New("anthropic model required")
	}
	base := strings.TrimRight(strings.TrimSpace(c.APIBase), "/")
	if base == "" {
		base = defaultAnthropicBase
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	reqBody := anthropicRequest{
		Model:     c.Model,
		MaxTokens: maxTokens,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	}
	body, err := marshalJSON(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("anthropic status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var out anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	for _, block := range out.Content {
		if strings.TrimSpace(block.Text) != "" {
			return block.Text, nil
		}
	}
	return "", errors.New("anthropic empty response")
}
