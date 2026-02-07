package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type RouterClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func (c *RouterClient) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResponse, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return ExecuteResponse{}, errors.New("base url required")
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return ExecuteResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/v1/tools:execute", bytes.NewReader(body))
	if err != nil {
		return ExecuteResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return ExecuteResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ExecuteResponse{}, errors.New("tool router status " + resp.Status)
	}
	var out ExecuteResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ExecuteResponse{}, err
	}
	return out, nil
}
