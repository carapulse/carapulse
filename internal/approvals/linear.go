package approvals

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultLinearBaseURL = "https://api.linear.app/graphql"

const (
	labelPending  = "approval:pending"
	labelApproved = "approval:approved"
	labelDenied   = "approval:denied"
	labelExpired  = "approval:expired"
)

type Issue struct {
	ID          string
	Title       string
	Description string
	Labels      []string
	CreatedAt   time.Time
}

type ApprovalClient interface {
	CreateApprovalIssue(ctx context.Context, planID string) (string, error)
	UpdateApprovalStatus(ctx context.Context, issueID, status string) error
	ListApprovalIssues(ctx context.Context) ([]Issue, error)
}

type LinearClient struct {
	BaseURL    string
	Token      string
	TeamID     string
	Client     *http.Client
	labelCache map[string]string
}

func NewLinearClient() *LinearClient {
	return &LinearClient{BaseURL: defaultLinearBaseURL}
}

func (c *LinearClient) CreateApprovalIssue(ctx context.Context, planID string) (string, error) {
	if planID == "" {
		return "", errors.New("plan id required")
	}
	if c.TeamID == "" {
		return "", errors.New("linear team id required")
	}
	labelID, err := c.labelID(ctx, labelPending)
	if err != nil {
		return "", err
	}
	query := `mutation($input: IssueCreateInput!) { issueCreate(input: $input) { issue { id } } }`
	vars := map[string]any{
		"input": map[string]any{
			"teamId":      c.TeamID,
			"title":       fmt.Sprintf("Approval required: %s", planID),
			"description": fmt.Sprintf("Approval required for plan.\n\nPlan ID: %s\n", planID),
			"labelIds":    []string{labelID},
		},
	}
	var resp struct {
		IssueCreate struct {
			Issue struct {
				ID string `json:"id"`
			} `json:"issue"`
		} `json:"issueCreate"`
	}
	if err := c.doGraphQL(ctx, query, vars, &resp); err != nil {
		return "", err
	}
	if resp.IssueCreate.Issue.ID == "" {
		return "", errors.New("missing issue id")
	}
	return resp.IssueCreate.Issue.ID, nil
}

func (c *LinearClient) UpdateApprovalStatus(ctx context.Context, issueID, status string) error {
	if issueID == "" {
		return errors.New("issue id required")
	}
	label, ok := labelForStatus(status)
	if !ok {
		return fmt.Errorf("unknown status: %s", status)
	}
	labelID, err := c.labelID(ctx, label)
	if err != nil {
		return err
	}
	query := `mutation($input: IssueUpdateInput!) { issueUpdate(input: $input) { success } }`
	vars := map[string]any{
		"input": map[string]any{
			"id":       issueID,
			"labelIds": []string{labelID},
		},
	}
	return c.doGraphQL(ctx, query, vars, nil)
}

func (c *LinearClient) ListApprovalIssues(ctx context.Context) ([]Issue, error) {
	query := `query($labels: [String!]!) { issues(first: 100, filter: { labels: { name: { in: $labels } } }) { nodes { id title description createdAt labels { nodes { name } } } } }`
	vars := map[string]any{"labels": []string{labelPending, labelApproved, labelDenied, labelExpired}}
	var resp struct {
		Issues struct {
			Nodes []struct {
				ID          string    `json:"id"`
				Title       string    `json:"title"`
				Description string    `json:"description"`
				CreatedAt   time.Time `json:"createdAt"`
				Labels      struct {
					Nodes []struct {
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"labels"`
			} `json:"nodes"`
		} `json:"issues"`
	}
	if err := c.doGraphQL(ctx, query, vars, &resp); err != nil {
		return nil, err
	}
	issues := make([]Issue, 0, len(resp.Issues.Nodes))
	for _, node := range resp.Issues.Nodes {
		labels := make([]string, 0, len(node.Labels.Nodes))
		for _, label := range node.Labels.Nodes {
			labels = append(labels, label.Name)
		}
		issues = append(issues, Issue{
			ID:          node.ID,
			Title:       node.Title,
			Description: node.Description,
			Labels:      labels,
			CreatedAt:   node.CreatedAt,
		})
	}
	return issues, nil
}

func (c *LinearClient) labelID(ctx context.Context, name string) (string, error) {
	if name == "" {
		return "", errors.New("label name required")
	}
	if c.labelCache == nil {
		c.labelCache = map[string]string{}
	}
	if id, ok := c.labelCache[name]; ok {
		return id, nil
	}
	query := `query($name: String!) { issueLabels(filter: { name: { eq: $name } }) { nodes { id name } } }`
	vars := map[string]any{"name": name}
	var resp struct {
		IssueLabels struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"issueLabels"`
	}
	if err := c.doGraphQL(ctx, query, vars, &resp); err != nil {
		return "", err
	}
	if len(resp.IssueLabels.Nodes) == 0 {
		return "", fmt.Errorf("label not found: %s", name)
	}
	id := resp.IssueLabels.Nodes[0].ID
	if id == "" {
		return "", errors.New("label id missing")
	}
	c.labelCache[name] = id
	return id, nil
}

func labelForStatus(status string) (string, bool) {
	switch status {
	case "pending":
		return labelPending, true
	case "approved":
		return labelApproved, true
	case "denied":
		return labelDenied, true
	case "expired":
		return labelExpired, true
	default:
		return "", false
	}
}

func approvalStatus(labels []string) string {
	if hasLabel(labels, labelApproved) {
		return "approved"
	}
	if hasLabel(labels, labelDenied) {
		return "denied"
	}
	if hasLabel(labels, labelExpired) {
		return "expired"
	}
	if hasLabel(labels, labelPending) {
		return "pending"
	}
	return ""
}

func hasLabel(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}

func extractPlanID(input string) (string, bool) {
	idx := strings.Index(input, "plan_")
	if idx == -1 {
		return "", false
	}
	start := idx
	end := idx + len("plan_")
	for end < len(input) {
		c := input[end]
		if c < '0' || c > '9' {
			break
		}
		end++
	}
	if end == start+len("plan_") {
		return "", false
	}
	return input[start:end], true
}

func (c *LinearClient) doGraphQL(ctx context.Context, query string, vars map[string]any, out any) error {
	if c.Token == "" {
		return errors.New("linear token required")
	}
	url := c.BaseURL
	if url == "" {
		url = defaultLinearBaseURL
	}
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 5 * time.Second}
	}
	payload, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", c.Token)
	resp, err := c.Client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("linear status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("linear error: %s", envelope.Errors[0].Message)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}
