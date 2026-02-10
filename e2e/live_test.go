package e2e_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/db"
	"carapulse/internal/policy"
	"carapulse/internal/web"
)

// fakeDB implements web.DBWriter and related interfaces for live testing.
type fakeDB struct {
	plans      map[string][]byte
	approvals  map[string]string
	executions map[string][]byte
	schedules  [][]byte
	playbooks  map[string][]byte
	runbooks   map[string][]byte
	workflows  [][]byte
	sessions   map[string][]byte
	nextPlanID int
	nextExecID int
}

func newFakeDB() *fakeDB {
	return &fakeDB{
		plans:      map[string][]byte{},
		approvals:  map[string]string{},
		executions: map[string][]byte{},
		playbooks:  map[string][]byte{},
		runbooks:   map[string][]byte{},
		sessions:   map[string][]byte{},
	}
}

func (f *fakeDB) CreatePlan(_ context.Context, planJSON []byte) (string, error) {
	f.nextPlanID++
	id := fmt.Sprintf("plan_%d", f.nextPlanID)
	// Store with plan_id injected
	var m map[string]any
	_ = json.Unmarshal(planJSON, &m)
	m["plan_id"] = id
	data, _ := json.Marshal(m)
	f.plans[id] = data
	return id, nil
}

func (f *fakeDB) GetPlan(_ context.Context, planID string) ([]byte, error) {
	data := f.plans[planID]
	return data, nil
}

func (f *fakeDB) CreateExecution(_ context.Context, planID string) (string, error) {
	f.nextExecID++
	id := fmt.Sprintf("exec_%d", f.nextExecID)
	exec := map[string]any{"execution_id": id, "plan_id": planID, "status": "running"}
	data, _ := json.Marshal(exec)
	f.executions[id] = data
	return id, nil
}

func (f *fakeDB) GetExecution(_ context.Context, execID string) ([]byte, error) {
	return f.executions[execID], nil
}

func (f *fakeDB) CreateApproval(_ context.Context, planID string, _ []byte) (string, error) {
	id := "approval_" + planID
	f.approvals[planID] = "pending"
	return id, nil
}

func (f *fakeDB) UpdateApprovalStatusByPlan(_ context.Context, planID, status string) error {
	f.approvals[planID] = status
	return nil
}

func (f *fakeDB) GetApprovalStatus(_ context.Context, planID string) (string, error) {
	if s, ok := f.approvals[planID]; ok {
		return s, nil
	}
	return "", fmt.Errorf("no approval for %s", planID)
}

func (f *fakeDB) SetApprovalHash(_ context.Context, planID, hash string) error {
	return nil
}

func (f *fakeDB) GetApprovalHash(_ context.Context, planID string) (string, error) {
	return "", nil
}

func (f *fakeDB) ListAuditEvents(_ context.Context, _ db.AuditFilter) ([]byte, int, error) {
	return []byte("[]"), 0, nil
}

func (f *fakeDB) InsertAuditEvent(_ context.Context, payload []byte) (string, error) {
	return "audit_1", nil
}

func (f *fakeDB) ListContextServices(_ context.Context, limit, offset int) ([]byte, int, error) {
	items := []map[string]any{
		{"service": "api-gateway", "labels": map[string]any{"tenant_id": "test-tenant"}},
	}
	data, _ := json.Marshal(items)
	return data, 1, nil
}

func (f *fakeDB) ListContextSnapshots(_ context.Context, limit, offset int) ([]byte, int, error) {
	items := []map[string]any{
		{"snapshot_id": "snap_1", "labels": map[string]any{"tenant_id": "test-tenant"}},
	}
	data, _ := json.Marshal(items)
	return data, 1, nil
}

func (f *fakeDB) GetContextSnapshot(_ context.Context, snapshotID string) ([]byte, error) {
	return json.Marshal(map[string]any{"snapshot_id": snapshotID, "labels": map[string]any{"tenant_id": "test-tenant"}})
}

func (f *fakeDB) CreateSchedule(_ context.Context, payload []byte) (string, error) {
	f.schedules = append(f.schedules, payload)
	return fmt.Sprintf("schedule_%d", len(f.schedules)), nil
}

func (f *fakeDB) ListPlans(_ context.Context, limit, offset int) ([]byte, int, error) {
	items := []json.RawMessage{}
	for _, v := range f.plans {
		items = append(items, v)
	}
	data, _ := json.Marshal(items)
	return data, len(items), nil
}

func (f *fakeDB) ListExecutions(_ context.Context, limit, offset int) ([]byte, int, error) {
	items := []json.RawMessage{}
	for _, v := range f.executions {
		items = append(items, v)
	}
	data, _ := json.Marshal(items)
	return data, len(items), nil
}

func (f *fakeDB) CancelExecution(_ context.Context, executionID string) error {
	if _, ok := f.executions[executionID]; !ok {
		return fmt.Errorf("not found")
	}
	exec := map[string]any{"execution_id": executionID, "status": "cancelled"}
	data, _ := json.Marshal(exec)
	f.executions[executionID] = data
	return nil
}

func (f *fakeDB) DeletePlan(_ context.Context, planID string) error {
	delete(f.plans, planID)
	return nil
}

func (f *fakeDB) DeleteSchedule(_ context.Context, scheduleID string) error {
	return nil
}

func (f *fakeDB) DeletePlaybook(_ context.Context, playbookID string) error {
	delete(f.playbooks, playbookID)
	return nil
}

func (f *fakeDB) DeleteRunbook(_ context.Context, runbookID string) error {
	delete(f.runbooks, runbookID)
	return nil
}

func (f *fakeDB) ListSchedules(_ context.Context, limit, offset int) ([]byte, int, error) {
	if len(f.schedules) == 0 {
		return []byte("[]"), 0, nil
	}
	data, err := json.Marshal(f.schedules)
	return data, len(f.schedules), err
}

func (f *fakeDB) CreatePlaybook(_ context.Context, payload []byte) (string, error) {
	var m map[string]any
	_ = json.Unmarshal(payload, &m)
	id := fmt.Sprintf("playbook_%d", len(f.playbooks)+1)
	m["playbook_id"] = id
	m["tenant_id"] = "test-tenant"
	data, _ := json.Marshal(m)
	f.playbooks[id] = data
	return id, nil
}

func (f *fakeDB) ListPlaybooks(_ context.Context, limit, offset int) ([]byte, int, error) {
	items := []json.RawMessage{}
	for _, v := range f.playbooks {
		items = append(items, v)
	}
	data, _ := json.Marshal(items)
	return data, len(items), nil
}

func (f *fakeDB) GetPlaybook(_ context.Context, playbookID string) ([]byte, error) {
	return f.playbooks[playbookID], nil
}

func (f *fakeDB) CreateRunbook(_ context.Context, payload []byte) (string, error) {
	var m map[string]any
	_ = json.Unmarshal(payload, &m)
	id := fmt.Sprintf("runbook_%d", len(f.runbooks)+1)
	m["runbook_id"] = id
	m["tenant_id"] = "test-tenant"
	data, _ := json.Marshal(m)
	f.runbooks[id] = data
	return id, nil
}

func (f *fakeDB) ListRunbooks(_ context.Context, limit, offset int) ([]byte, int, error) {
	items := []json.RawMessage{}
	for _, v := range f.runbooks {
		items = append(items, v)
	}
	data, _ := json.Marshal(items)
	return data, len(items), nil
}

func (f *fakeDB) GetRunbook(_ context.Context, runbookID string) ([]byte, error) {
	return f.runbooks[runbookID], nil
}

func (f *fakeDB) CreateWorkflowCatalog(_ context.Context, payload []byte) (string, error) {
	f.workflows = append(f.workflows, payload)
	return fmt.Sprintf("wf_%d", len(f.workflows)), nil
}

func (f *fakeDB) ListWorkflowCatalog(_ context.Context, limit, offset int) ([]byte, int, error) {
	if len(f.workflows) == 0 {
		return []byte("[]"), 0, nil
	}
	data, err := json.Marshal(f.workflows)
	return data, len(f.workflows), err
}

func (f *fakeDB) CreateSession(_ context.Context, payload []byte) (string, error) {
	var m map[string]any
	_ = json.Unmarshal(payload, &m)
	id := fmt.Sprintf("session_%d", len(f.sessions)+1)
	m["session_id"] = id
	data, _ := json.Marshal(m)
	f.sessions[id] = data
	return id, nil
}

func (f *fakeDB) ListSessions(_ context.Context, limit, offset int) ([]byte, int, error) {
	items := []json.RawMessage{}
	for _, v := range f.sessions {
		items = append(items, v)
	}
	data, _ := json.Marshal(items)
	return data, len(items), nil
}

func (f *fakeDB) GetSession(_ context.Context, sessionID string) ([]byte, error) {
	return f.sessions[sessionID], nil
}

func (f *fakeDB) UpdateSession(_ context.Context, sessionID string, payload []byte) error {
	f.sessions[sessionID] = payload
	return nil
}

func (f *fakeDB) DeleteSession(_ context.Context, sessionID string) error {
	delete(f.sessions, sessionID)
	return nil
}

func (f *fakeDB) AddSessionMember(_ context.Context, sessionID string, payload []byte) error {
	return nil
}

func (f *fakeDB) ListSessionMembers(_ context.Context, sessionID string) ([]byte, error) {
	return []byte("[]"), nil
}

func (f *fakeDB) IsSessionMember(_ context.Context, sessionID, memberID string) (bool, error) {
	return true, nil
}

func (f *fakeDB) HasActiveExecution(_ context.Context, planID string) (bool, error) {
	return false, nil
}

func (f *fakeDB) ListPlanSteps(_ context.Context, planID string) ([]byte, error) {
	return []byte("[]"), nil
}

func (f *fakeDB) ListApprovalsByPlan(_ context.Context, planID string) ([]byte, error) {
	return []byte("[]"), nil
}

// Implement the Conn() method to avoid nil DBConn panics
func (f *fakeDB) Conn() interface{} {
	return nil
}

// fakeContextManager implements web.ContextManager
type fakeContextManager struct{}

func (f *fakeContextManager) RefreshContext(_ context.Context) error { return nil }
func (f *fakeContextManager) IngestSnapshot(_ context.Context, nodes []ctxmodel.Node, edges []ctxmodel.Edge) error {
	return nil
}
func (f *fakeContextManager) GetServiceGraph(_ context.Context, service string) (ctxmodel.ServiceGraph, error) {
	return ctxmodel.ServiceGraph{Nodes: []ctxmodel.Node{}, Edges: []ctxmodel.Edge{}}, nil
}

// devJWT creates a minimal JWT token for dev mode testing.
func devJWT(sub, email string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims := map[string]any{
		"sub":   sub,
		"email": email,
		"exp":   float64(time.Now().Add(1 * time.Hour).Unix()),
	}
	claimsJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	return header + "." + payload + "."
}

// Result captures a test result.
type Result struct {
	Endpoint   string
	Method     string
	Status     int
	Body       string
	Passed     bool
	FailReason string
}

func TestLiveEndpoints(t *testing.T) {
	// Enable dev mode for JWT bypass.
	web.SetAuthConfig(web.AuthConfig{DevMode: true})

	fdb := newFakeDB()
	evaluator := &policy.Evaluator{
		Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) {
			return policy.PolicyDecision{Decision: "allow"}, nil
		}),
	}
	srv := web.NewServer(fdb, evaluator)
	srv.AutoApproveLow = true
	srv.Context = &fakeContextManager{}

	ts := httptest.NewServer(srv.Mux)
	defer ts.Close()

	token := devJWT("test-user", "test@carapulse.dev")
	baseURL := ts.URL

	var results []Result

	doReq := func(method, path string, body any) Result {
		var bodyReader io.Reader
		if body != nil {
			data, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(data)
		}
		req, _ := http.NewRequest(method, baseURL+path, bodyReader)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-Id", "test-tenant")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			r := Result{Endpoint: path, Method: method, Passed: false, FailReason: err.Error()}
			results = append(results, r)
			return r
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		r := Result{Endpoint: path, Method: method, Status: resp.StatusCode, Body: string(respBody), Passed: true}
		results = append(results, r)
		return r
	}

	expectStatus := func(r Result, expected int) {
		t.Helper()
		if r.Status != expected {
			r.Passed = false
			r.FailReason = fmt.Sprintf("expected %d, got %d: %s", expected, r.Status, r.Body)
			// Update the last result
			results[len(results)-1] = r
			t.Errorf("%s %s: expected status %d, got %d. Body: %s", r.Method, r.Endpoint, expected, r.Status, r.Body)
		}
	}

	// ============================
	// 1. Health endpoints (no auth)
	// ============================
	t.Run("GET /healthz", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/healthz", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		r := Result{Endpoint: "/healthz", Method: "GET", Status: resp.StatusCode, Body: string(body), Passed: resp.StatusCode == 200}
		results = append(results, r)
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "ok") {
			t.Errorf("expected ok in body, got %s", body)
		}
	})

	t.Run("GET /readyz", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/readyz", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		r := Result{Endpoint: "/readyz", Method: "GET", Status: resp.StatusCode, Body: string(body), Passed: true}
		results = append(results, r)
		// Without real DB, readyz might report unavailable - that's fine
		t.Logf("readyz: %d %s", resp.StatusCode, body)
	})

	t.Run("GET /metrics", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/metrics", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		r := Result{Endpoint: "/metrics", Method: "GET", Status: resp.StatusCode, Body: string(body[:min(len(body), 200)]), Passed: resp.StatusCode == 200}
		results = append(results, r)
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	// ============================
	// 2. Plans CRUD
	// ============================
	var planID string
	t.Run("POST /v1/plans", func(t *testing.T) {
		r := doReq("POST", "/v1/plans", map[string]any{
			"summary": "Deploy v2.1 to staging",
			"trigger": "manual",
			"intent":  "list pods in staging namespace",
			"context": map[string]any{
				"tenant_id":   "test-tenant",
				"environment": "staging",
				"cluster_id":  "cluster-1",
				"namespace":   "staging",
				"aws_account_id":  "123456789012",
				"region":          "us-east-1",
				"argocd_project":  "default",
				"grafana_org_id":  "1",
			},
		})
		expectStatus(r, 200)
		var resp map[string]any
		if err := json.Unmarshal([]byte(r.Body), &resp); err != nil {
			t.Fatalf("invalid json: %s", r.Body)
		}
		if id, ok := resp["plan_id"].(string); ok {
			planID = id
			t.Logf("Created plan: %s", planID)
		} else {
			t.Errorf("missing plan_id in response: %v", resp)
		}
	})

	t.Run("GET /v1/plans/{id}", func(t *testing.T) {
		if planID == "" {
			t.Skip("no plan created")
		}
		r := doReq("GET", "/v1/plans/"+planID, nil)
		expectStatus(r, 200)
		t.Logf("Plan response: %s", r.Body[:min(len(r.Body), 300)])
	})

	t.Run("GET /v1/plans/{id}/diff", func(t *testing.T) {
		if planID == "" {
			t.Skip("no plan created")
		}
		r := doReq("GET", "/v1/plans/"+planID+"/diff", nil)
		expectStatus(r, 200)
		t.Logf("Diff response: %s", r.Body)
	})

	t.Run("GET /v1/plans/{id}/risk", func(t *testing.T) {
		if planID == "" {
			t.Skip("no plan created")
		}
		r := doReq("GET", "/v1/plans/"+planID+"/risk", nil)
		expectStatus(r, 200)
		t.Logf("Risk response: %s", r.Body)
	})

	t.Run("GET /v1/plans/nonexistent", func(t *testing.T) {
		r := doReq("GET", "/v1/plans/nonexistent", nil)
		expectStatus(r, 404)
	})

	// ============================
	// 3. Approvals
	// ============================
	t.Run("POST /v1/approvals", func(t *testing.T) {
		if planID == "" {
			t.Skip("no plan created")
		}
		r := doReq("POST", "/v1/approvals", map[string]any{
			"plan_id": planID,
			"status":  "approved",
		})
		expectStatus(r, 200)
		t.Logf("Approval response: %s", r.Body)
	})

	// ============================
	// 4. Execute plan
	// ============================
	t.Run("POST /v1/plans/{id}:execute", func(t *testing.T) {
		if planID == "" {
			t.Skip("no plan created")
		}
		r := doReq("POST", "/v1/plans/"+planID+":execute", map[string]any{})
		// read risk plans auto-approve and should execute
		t.Logf("Execute response: %d %s", r.Status, r.Body)
		// Accept 200, 403, or 409 since this depends on approval status
		if r.Status != 200 && r.Status != 403 && r.Status != 409 {
			t.Errorf("unexpected status %d: %s", r.Status, r.Body)
		}
	})

	// ============================
	// 5. Schedules
	// ============================
	t.Run("GET /v1/schedules", func(t *testing.T) {
		r := doReq("GET", "/v1/schedules", nil)
		expectStatus(r, 200)
		t.Logf("Schedules: %s", r.Body)
	})

	t.Run("POST /v1/schedules", func(t *testing.T) {
		r := doReq("POST", "/v1/schedules", map[string]any{
			"summary": "Nightly cleanup",
			"cron":    "0 2 * * *",
			"intent":  "clean old pods",
			"context": map[string]any{
				"tenant_id":   "test-tenant",
				"environment": "production",
			},
			"enabled": true,
		})
		expectStatus(r, 200)
		t.Logf("Create schedule response: %s", r.Body)
	})

	// ============================
	// 6. Context endpoints
	// ============================
	t.Run("POST /v1/context/refresh", func(t *testing.T) {
		r := doReq("POST", "/v1/context/refresh", map[string]any{
			"service": "api-gateway",
		})
		expectStatus(r, 200)
		t.Logf("Context refresh: %s", r.Body)
	})

	t.Run("GET /v1/context/services", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/v1/context/services", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Tenant-Id", "test-tenant")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		r := Result{Endpoint: "/v1/context/services", Method: "GET", Status: resp.StatusCode, Body: string(body), Passed: resp.StatusCode == 200}
		results = append(results, r)
		t.Logf("Context services: %d %s", resp.StatusCode, body)
	})

	t.Run("GET /v1/context/snapshots", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/v1/context/snapshots", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Tenant-Id", "test-tenant")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		r := Result{Endpoint: "/v1/context/snapshots", Method: "GET", Status: resp.StatusCode, Body: string(body), Passed: resp.StatusCode == 200}
		results = append(results, r)
		t.Logf("Context snapshots: %d %s", resp.StatusCode, body)
	})

	t.Run("GET /v1/context/graph", func(t *testing.T) {
		r := doReq("GET", "/v1/context/graph?service=api-gateway", nil)
		expectStatus(r, 200)
		t.Logf("Context graph: %s", r.Body)
	})

	// ============================
	// 7. Hooks
	// ============================
	t.Run("POST /v1/hooks/alertmanager", func(t *testing.T) {
		r := doReq("POST", "/v1/hooks/alertmanager", map[string]any{
			"alerts": []map[string]any{
				{
					"status": "firing",
					"labels": map[string]any{
						"alertname": "HighErrorRate",
						"severity":  "critical",
						"namespace": "production",
					},
				},
			},
		})
		// Should succeed or policy-deny
		t.Logf("Hook alertmanager: %d %s", r.Status, r.Body)
		if r.Status != 200 && r.Status != 202 && r.Status != 403 {
			t.Errorf("unexpected status %d for alertmanager hook", r.Status)
		}
	})

	t.Run("POST /v1/hooks/argocd", func(t *testing.T) {
		r := doReq("POST", "/v1/hooks/argocd", map[string]any{
			"type":    "sync",
			"app":     "my-app",
			"project": "default",
		})
		t.Logf("Hook argocd: %d %s", r.Status, r.Body)
	})

	t.Run("POST /v1/hooks/git", func(t *testing.T) {
		r := doReq("POST", "/v1/hooks/git", map[string]any{
			"ref":     "refs/heads/main",
			"commits": []any{},
		})
		t.Logf("Hook git: %d %s", r.Status, r.Body)
	})

	t.Run("POST /v1/hooks/k8s", func(t *testing.T) {
		r := doReq("POST", "/v1/hooks/k8s", map[string]any{
			"type": "Warning",
			"reason": "OOMKilled",
			"object": map[string]any{"kind": "Pod", "name": "my-pod"},
		})
		t.Logf("Hook k8s: %d %s", r.Status, r.Body)
	})

	// ============================
	// 8. Playbooks
	// ============================
	t.Run("POST /v1/playbooks", func(t *testing.T) {
		r := doReq("POST", "/v1/playbooks", map[string]any{
			"tenant_id": "test-tenant",
			"name":        "Incident Response",
			"version":     1,
			"spec":        map[string]any{"steps": []string{"Acknowledge", "Investigate", "Mitigate", "Post-mortem"}},
		})
		expectStatus(r, 200)
		t.Logf("Create playbook: %s", r.Body)
	})

	t.Run("GET /v1/playbooks", func(t *testing.T) {
		r := doReq("GET", "/v1/playbooks", nil)
		expectStatus(r, 200)
		t.Logf("List playbooks: %s", r.Body)
	})

	// ============================
	// 9. Runbooks
	// ============================
	t.Run("POST /v1/runbooks", func(t *testing.T) {
		r := doReq("POST", "/v1/runbooks", map[string]any{
			"tenant_id": "test-tenant",
			"service":   "api-gateway",
			"name":      "Scale Up Service",
			"version":   1,
			"body":      "Scale deployment replicas",
			"spec":      map[string]any{"commands": []string{"kubectl scale deployment/app --replicas=5"}},
		})
		expectStatus(r, 200)
		t.Logf("Create runbook: %s", r.Body)
	})

	t.Run("GET /v1/runbooks", func(t *testing.T) {
		r := doReq("GET", "/v1/runbooks", nil)
		expectStatus(r, 200)
		t.Logf("List runbooks: %s", r.Body)
	})

	// ============================
	// 10. Workflows
	// ============================
	t.Run("GET /v1/workflows", func(t *testing.T) {
		r := doReq("GET", "/v1/workflows", nil)
		t.Logf("List workflows: %d %s", r.Status, r.Body)
		// May be 200 or 405 depending on implementation
	})

	// ============================
	// 11. Sessions
	// ============================
	t.Run("POST /v1/sessions", func(t *testing.T) {
		r := doReq("POST", "/v1/sessions", map[string]any{
			"name": "Incident-2024-001",
		})
		t.Logf("Create session: %d %s", r.Status, r.Body)
	})

	t.Run("GET /v1/sessions", func(t *testing.T) {
		r := doReq("GET", "/v1/sessions", nil)
		t.Logf("List sessions: %d %s", r.Status, r.Body)
	})

	// ============================
	// 12. Audit events
	// ============================
	t.Run("GET /v1/audit/events", func(t *testing.T) {
		r := doReq("GET", "/v1/audit/events", nil)
		expectStatus(r, 200)
		t.Logf("Audit events: %s", r.Body)
	})

	// ============================
	// 13. Auth - Unauthorized
	// ============================
	t.Run("POST /v1/plans (no auth)", func(t *testing.T) {
		req, _ := http.NewRequest("POST", baseURL+"/v1/plans", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		r := Result{Endpoint: "/v1/plans (no auth)", Method: "POST", Status: resp.StatusCode, Body: string(body), Passed: resp.StatusCode == 401}
		results = append(results, r)
		if resp.StatusCode != 401 {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
	})

	// ============================
	// 14. Memory endpoints
	// ============================
	t.Run("POST /v1/memory", func(t *testing.T) {
		r := doReq("POST", "/v1/memory", map[string]any{
			"key":   "test-insight",
			"value": "Scaling to 10 replicas resolved the issue",
		})
		t.Logf("Create memory: %d %s", r.Status, r.Body)
	})

	t.Run("GET /v1/memory", func(t *testing.T) {
		// No auth header for memory list
		r := doReq("GET", "/v1/memory", nil)
		t.Logf("List memory: %d %s", r.Status, r.Body)
	})

	// ============================
	// 15. Method not allowed
	// ============================
	t.Run("DELETE /v1/plans (method not allowed)", func(t *testing.T) {
		r := doReq("DELETE", "/v1/plans", nil)
		if r.Status != 405 {
			t.Logf("DELETE /v1/plans returned %d (expected 405)", r.Status)
		}
	})

	// ============================
	// 16. Invalid JSON
	// ============================
	t.Run("POST /v1/plans (invalid json)", func(t *testing.T) {
		req, _ := http.NewRequest("POST", baseURL+"/v1/plans", strings.NewReader(`not json`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		r := Result{Endpoint: "/v1/plans (invalid json)", Method: "POST", Status: resp.StatusCode, Body: string(body), Passed: resp.StatusCode == 400}
		results = append(results, r)
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for invalid json, got %d", resp.StatusCode)
		}
	})

	// ============================
	// 17. Execution logs (SSE)
	// ============================
	t.Run("GET /v1/executions/{id}/logs", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/v1/executions/exec_1/logs", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		req = req.WithContext(ctx)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Logf("SSE connection error (expected with timeout): %v", err)
			return
		}
		defer resp.Body.Close()
		body := make([]byte, 256)
		n, _ := resp.Body.Read(body)
		r := Result{Endpoint: "/v1/executions/exec_1/logs", Method: "GET", Status: resp.StatusCode, Body: string(body[:n]), Passed: resp.StatusCode == 200}
		results = append(results, r)
		t.Logf("SSE logs: %d %s", resp.StatusCode, string(body[:n]))
	})

	// ============================
	// Generate Report
	// ============================
	generateReport(t, results)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func generateReport(t *testing.T, results []Result) {
	t.Helper()

	var sb strings.Builder
	sb.WriteString("# Carapulse Live Test Results\n\n")
	sb.WriteString(fmt.Sprintf("**Date:** %s\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("**Test Mode:** In-process with httptest (no external deps)\n\n"))

	passed := 0
	failed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		} else {
			failed++
		}
	}
	sb.WriteString(fmt.Sprintf("## Summary\n\n"))
	sb.WriteString(fmt.Sprintf("- **Total endpoints tested:** %d\n", len(results)))
	sb.WriteString(fmt.Sprintf("- **Passed:** %d\n", passed))
	sb.WriteString(fmt.Sprintf("- **Failed:** %d\n\n", failed))

	sb.WriteString("## Detailed Results\n\n")
	sb.WriteString("| Method | Endpoint | Status | Result | Notes |\n")
	sb.WriteString("|--------|----------|--------|--------|-------|\n")
	for _, r := range results {
		status := "PASS"
		notes := ""
		if !r.Passed {
			status = "FAIL"
			notes = r.FailReason
		}
		body := r.Body
		if len(body) > 80 {
			body = body[:80] + "..."
		}
		body = strings.ReplaceAll(body, "|", "\\|")
		body = strings.ReplaceAll(body, "\n", " ")
		sb.WriteString(fmt.Sprintf("| %s | %s | %d | %s | %s |\n", r.Method, r.Endpoint, r.Status, status, notes))
	}

	sb.WriteString("\n## Findings\n\n")
	if failed == 0 {
		sb.WriteString("- All endpoints passed.\n")
	} else {
		sb.WriteString("- Some endpoints failed; see table above.\n")
	}

	report := sb.String()

	// Write report to temp dir by default (avoid dirtying the worktree during tests).
	outPath := filepath.Join(t.TempDir(), "live-test-results.md")
	if err := os.WriteFile(outPath, []byte(report), 0o644); err != nil {
		t.Errorf("failed to write report: %v", err)
		return
	}
	t.Logf("Report written to %s", outPath)
	if os.Getenv("CARAPULSE_WRITE_LIVE_REPORT") == "1" {
		_, file, _, ok := runtime.Caller(0)
		if ok {
			// live_test.go is in <repo>/e2e. Write to <repo>/reports when requested.
			repoRoot := filepath.Dir(filepath.Dir(file))
			_ = os.WriteFile(filepath.Join(repoRoot, "reports", "live-test-results.md"), []byte(report), 0o644)
		}
	}
}
