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

const defaultOpenAIBase = "https://api.openai.com"

type OpenAIClient struct {
	APIBase    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

func (c *OpenAIClient) Complete(prompt string, maxTokens int) (string, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return "", errors.New("openai api key required")
	}
	if strings.TrimSpace(c.Model) == "" {
		return "", errors.New("openai model required")
	}
	base := strings.TrimRight(strings.TrimSpace(c.APIBase), "/")
	if base == "" {
		base = defaultOpenAIBase
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	reqBody := openAIRequest{
		Model:     c.Model,
		Messages:  []openAIMessage{{Role: "user", Content: prompt}},
		MaxTokens: maxTokens,
	}
	body, err := marshalJSON(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("openai status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var out openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", errors.New("openai empty response")
	}
	return out.Choices[0].Message.Content, nil
}
