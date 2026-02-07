package chatops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"carapulse/internal/web"
)

type HTTPGatewayClient struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func (c *HTTPGatewayClient) CreatePlan(ctx context.Context, req web.PlanCreateRequest) (string, error) {
	var resp planCreateResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/plans", req, &resp); err != nil {
		return "", err
	}
	if resp.PlanID == "" {
		resp.PlanID = resp.Plan.PlanID
	}
	if resp.PlanID == "" {
		return "", fmt.Errorf("missing plan_id")
	}
	return resp.PlanID, nil
}

func (c *HTTPGatewayClient) CreateApproval(ctx context.Context, planID, status, note string) error {
	req := web.ApprovalCreateRequest{PlanID: planID, Status: status, ApproverNote: note}
	return c.doJSON(ctx, http.MethodPost, "/v1/approvals", req, &struct{}{})
}

func (c *HTTPGatewayClient) GetExecution(ctx context.Context, executionID string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, "/v1/executions/"+executionID, nil)
}

func (c *HTTPGatewayClient) ListAudit(ctx context.Context, query url.Values) ([]byte, error) {
	path := "/v1/audit/events"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

type planCreateResponse struct {
	PlanID string `json:"plan_id"`
	Plan   struct {
		PlanID string `json:"plan_id"`
	} `json:"plan"`
}

func (c *HTTPGatewayClient) doJSON(ctx context.Context, method, path string, req any, out any) error {
	respBytes, err := c.doRequest(ctx, method, path, req)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBytes, out); err != nil {
		return err
	}
	return nil
}

func (c *HTTPGatewayClient) doRequest(ctx context.Context, method, path string, req any) ([]byte, error) {
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 5 * time.Second}
	}
	var body io.Reader
	if req != nil {
		data, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	rawQuery := ""
	if idx := strings.Index(path, "?"); idx != -1 {
		rawQuery = path[idx+1:]
		path = path[:idx]
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + path
	u.RawQuery = rawQuery
	request, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if req != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gateway status %d: %s", resp.StatusCode, string(payload))
	}
	return io.ReadAll(resp.Body)
}
