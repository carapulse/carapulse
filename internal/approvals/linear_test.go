package approvals

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("read")
}

func (errReadCloser) Close() error {
	return nil
}

func newResponse(status int, body io.ReadCloser) *http.Response {
	return &http.Response{StatusCode: status, Body: body, Header: make(http.Header)}
}

func TestLinearClientCreateApprovalIssue(t *testing.T) {
	var requests int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if got := r.Header.Get("Authorization"); got != "token" {
			t.Fatalf("auth: %s", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		query, _ := payload["query"].(string)
		vars, _ := payload["variables"].(map[string]any)
		switch {
		case strings.Contains(query, "issueLabels"):
			if vars["name"] != labelPending {
				t.Fatalf("label: %v", vars["name"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"issueLabels": map[string]any{
						"nodes": []any{map[string]any{"id": "label_pending", "name": labelPending}},
					},
				},
			})
		case strings.Contains(query, "issueCreate"):
			input, _ := vars["input"].(map[string]any)
			if input["teamId"] != "team" {
				t.Fatalf("team: %v", input["teamId"])
			}
			labelIDs, _ := input["labelIds"].([]any)
			if len(labelIDs) != 1 || labelIDs[0] != "label_pending" {
				t.Fatalf("labels: %v", labelIDs)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"issueCreate": map[string]any{
						"issue": map[string]any{"id": "issue_1"},
					},
				},
			})
		default:
			t.Fatalf("query: %s", query)
		}
	}))
	defer ts.Close()

	client := NewLinearClient()
	client.BaseURL = ts.URL
	client.Token = "token"
	client.TeamID = "team"
	if _, err := client.CreateApprovalIssue(context.Background(), "plan_1"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := client.CreateApprovalIssue(context.Background(), "plan_2"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 3 {
		t.Fatalf("requests: %d", got)
	}
}

func TestLinearClientCreateApprovalIssueMissingPlan(t *testing.T) {
	client := NewLinearClient()
	client.Token = "token"
	client.TeamID = "team"
	if _, err := client.CreateApprovalIssue(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLinearClientCreateApprovalIssueMissingToken(t *testing.T) {
	client := NewLinearClient()
	client.TeamID = "team"
	if _, err := client.CreateApprovalIssue(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLinearClientCreateApprovalIssueMissingTeam(t *testing.T) {
	client := NewLinearClient()
	client.Token = "token"
	if _, err := client.CreateApprovalIssue(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLinearClientCreateApprovalIssueMissingID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		query, _ := payload["query"].(string)
		if strings.Contains(query, "issueLabels") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"issueLabels": map[string]any{
						"nodes": []any{map[string]any{"id": "label_pending", "name": labelPending}},
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"issueCreate": map[string]any{
					"issue": map[string]any{"id": ""},
				},
			},
		})
	}))
	defer ts.Close()

	client := NewLinearClient()
	client.BaseURL = ts.URL
	client.Token = "token"
	client.TeamID = "team"
	if _, err := client.CreateApprovalIssue(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLinearClientCreateApprovalIssueGraphQLError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		query, _ := payload["query"].(string)
		if strings.Contains(query, "issueLabels") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"issueLabels": map[string]any{
						"nodes": []any{map[string]any{"id": "label_pending", "name": labelPending}},
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{map[string]any{"message": "boom"}},
		})
	}))
	defer ts.Close()

	client := NewLinearClient()
	client.BaseURL = ts.URL
	client.Token = "token"
	client.TeamID = "team"
	if _, err := client.CreateApprovalIssue(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLinearClientUpdateApprovalStatusInvalid(t *testing.T) {
	client := NewLinearClient()
	if err := client.UpdateApprovalStatus(context.Background(), "issue", "nope"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLinearClientUpdateApprovalStatusMissingIssue(t *testing.T) {
	client := NewLinearClient()
	if err := client.UpdateApprovalStatus(context.Background(), "", "approved"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLinearClientUpdateApprovalStatusLabelMissing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"issueLabels": map[string]any{
					"nodes": []any{},
				},
			},
		})
	}))
	defer ts.Close()

	client := NewLinearClient()
	client.BaseURL = ts.URL
	client.Token = "token"
	if err := client.UpdateApprovalStatus(context.Background(), "issue", "approved"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLinearClientUpdateApprovalStatusSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		query, _ := payload["query"].(string)
		if strings.Contains(query, "issueLabels") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"issueLabels": map[string]any{
						"nodes": []any{map[string]any{"id": "label_approved", "name": labelApproved}},
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"issueUpdate": map[string]any{"success": true},
			},
		})
	}))
	defer ts.Close()

	client := NewLinearClient()
	client.BaseURL = ts.URL
	client.Token = "token"
	if err := client.UpdateApprovalStatus(context.Background(), "issue", "approved"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestLinearClientUpdateApprovalStatusGraphQLError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		query, _ := payload["query"].(string)
		if strings.Contains(query, "issueLabels") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"issueLabels": map[string]any{
						"nodes": []any{map[string]any{"id": "label_approved", "name": labelApproved}},
					},
				},
			})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("fail"))
	}))
	defer ts.Close()

	client := NewLinearClient()
	client.BaseURL = ts.URL
	client.Token = "token"
	if err := client.UpdateApprovalStatus(context.Background(), "issue", "approved"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLinearClientListApprovalIssues(t *testing.T) {
	now := time.Now().UTC()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"issues": map[string]any{
					"nodes": []any{
						map[string]any{
							"id":          "issue_1",
							"title":       "Approval required: plan_1",
							"description": "Plan ID: plan_1",
							"createdAt":   now.Format(time.RFC3339),
							"labels": map[string]any{
								"nodes": []any{map[string]any{"name": labelApproved}},
							},
						},
					},
				},
			},
		})
	}))
	defer ts.Close()

	client := NewLinearClient()
	client.BaseURL = ts.URL
	client.Token = "token"
	issues, err := client.ListApprovalIssues(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(issues) != 1 || issues[0].ID != "issue_1" {
		t.Fatalf("issues: %+v", issues)
	}
	if issues[0].Labels[0] != labelApproved {
		t.Fatalf("labels: %v", issues[0].Labels)
	}
}

func TestLinearClientListApprovalIssuesError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{map[string]any{"message": "boom"}},
		})
	}))
	defer ts.Close()

	client := NewLinearClient()
	client.BaseURL = ts.URL
	client.Token = "token"
	if _, err := client.ListApprovalIssues(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLinearClientLabelIDErrors(t *testing.T) {
	client := NewLinearClient()
	client.Token = "token"
	if _, err := client.labelID(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"issueLabels": map[string]any{
					"nodes": []any{map[string]any{"id": "", "name": labelPending}},
				},
			},
		})
	}))
	defer ts.Close()

	client.BaseURL = ts.URL
	client.labelCache = map[string]string{}
	if _, err := client.labelID(context.Background(), labelPending); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLabelForStatus(t *testing.T) {
	cases := []struct {
		status string
		label  string
		ok     bool
	}{
		{"pending", labelPending, true},
		{"approved", labelApproved, true},
		{"denied", labelDenied, true},
		{"expired", labelExpired, true},
		{"nope", "", false},
	}
	for _, c := range cases {
		label, ok := labelForStatus(c.status)
		if ok != c.ok || label != c.label {
			t.Fatalf("status %s -> %s %v", c.status, label, ok)
		}
	}
}

func TestLinearClientDoGraphQLErrors(t *testing.T) {
	t.Run("token", func(t *testing.T) {
		client := NewLinearClient()
		if err := client.doGraphQL(context.Background(), "query", nil, nil); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("marshal", func(t *testing.T) {
		client := NewLinearClient()
		client.Token = "token"
		vars := map[string]any{"bad": func() {}}
		if err := client.doGraphQL(context.Background(), "query", vars, nil); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("bad-url", func(t *testing.T) {
		client := NewLinearClient()
		client.Token = "token"
		client.BaseURL = "http://[::1"
		if err := client.doGraphQL(context.Background(), "query", nil, nil); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("do-error", func(t *testing.T) {
		client := NewLinearClient()
		client.Token = "token"
		client.BaseURL = "http://example.com"
		client.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})}
		if err := client.doGraphQL(context.Background(), "query", nil, nil); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("read-error", func(t *testing.T) {
		client := NewLinearClient()
		client.Token = "token"
		client.BaseURL = "http://example.com"
		client.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return newResponse(http.StatusOK, errReadCloser{}), nil
		})}
		if err := client.doGraphQL(context.Background(), "query", nil, nil); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("status", func(t *testing.T) {
		client := NewLinearClient()
		client.Token = "token"
		client.BaseURL = "http://example.com"
		client.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return newResponse(http.StatusInternalServerError, io.NopCloser(strings.NewReader("fail"))), nil
		})}
		if err := client.doGraphQL(context.Background(), "query", nil, nil); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("bad-json", func(t *testing.T) {
		client := NewLinearClient()
		client.Token = "token"
		client.BaseURL = "http://example.com"
		client.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return newResponse(http.StatusOK, io.NopCloser(strings.NewReader("nope"))), nil
		})}
		if err := client.doGraphQL(context.Background(), "query", nil, nil); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("graphql", func(t *testing.T) {
		client := NewLinearClient()
		client.Token = "token"
		client.BaseURL = "http://example.com"
		client.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return newResponse(http.StatusOK, io.NopCloser(strings.NewReader(`{"errors":[{"message":"boom"}]}`))), nil
		})}
		if err := client.doGraphQL(context.Background(), "query", nil, nil); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestLinearClientDoGraphQLDefaultURL(t *testing.T) {
	client := NewLinearClient()
	client.BaseURL = ""
	client.Token = "token"
	client.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != defaultLinearBaseURL {
			return newResponse(http.StatusBadRequest, io.NopCloser(strings.NewReader("bad url"))), nil
		}
		return newResponse(http.StatusOK, io.NopCloser(strings.NewReader(`{"data":{}}`))), nil
	})}
	if err := client.doGraphQL(context.Background(), "query", nil, nil); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestLinearClientDoGraphQLSuccess(t *testing.T) {
	client := NewLinearClient()
	client.Token = "token"
	client.BaseURL = "http://example.com"
	client.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return newResponse(http.StatusOK, io.NopCloser(strings.NewReader(`{"data":{"issueLabels":{"nodes":[{"id":"label_pending"}]}}}`))), nil
	})}
	var out struct {
		IssueLabels struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
		} `json:"issueLabels"`
	}
	if err := client.doGraphQL(context.Background(), "query", nil, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.IssueLabels.Nodes) != 1 || out.IssueLabels.Nodes[0].ID != "label_pending" {
		t.Fatalf("out: %+v", out)
	}
}
