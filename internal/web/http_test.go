package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/db"
	"carapulse/internal/policy"
)

type fakeDB struct {
	lastPlan       []byte
	planID         string
	execID         string
	updatePlan     string
	updateStatus   string
	approvalStatus string
	approvalErr    error
}

func (f *fakeDB) CreatePlan(ctx context.Context, planJSON []byte) (string, error) {
	f.lastPlan = planJSON
	f.planID = "plan_1"
	return f.planID, nil
}

func (f *fakeDB) GetPlan(ctx context.Context, planID string) ([]byte, error) {
	if planID != f.planID {
		return nil, nil
	}
	return f.lastPlan, nil
}

func (f *fakeDB) CreateExecution(ctx context.Context, planID string) (string, error) {
	f.execID = "exec_1"
	return f.execID, nil
}

func (f *fakeDB) CreateApproval(ctx context.Context, planID string, payload []byte) (string, error) {
	return "approval_1", nil
}

func (f *fakeDB) GetExecution(ctx context.Context, execID string) ([]byte, error) {
	return []byte(`{"execution_id":"exec_1"}`), nil
}

func (f *fakeDB) UpdateApprovalStatusByPlan(ctx context.Context, planID, status string) error {
	f.updatePlan = planID
	f.updateStatus = status
	return nil
}

func (f *fakeDB) GetApprovalStatus(ctx context.Context, planID string) (string, error) {
	if f.approvalErr != nil {
		return "", f.approvalErr
	}
	if f.approvalStatus != "" {
		return f.approvalStatus, nil
	}
	return "approved", nil
}

func (f *fakeDB) ListAuditEvents(ctx context.Context, filter db.AuditFilter) ([]byte, error) {
	return []byte("[]"), nil
}

func (f *fakeDB) ListContextServices(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (f *fakeDB) IsSessionMember(ctx context.Context, sessionID, memberID string) (bool, error) {
	return true, nil
}

func (f *fakeDB) CreateSchedule(ctx context.Context, payload []byte) (string, error) {
	return "schedule_1", nil
}

func (f *fakeDB) ListSchedules(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (f *fakeDB) CreatePlaybook(ctx context.Context, payload []byte) (string, error) {
	return "playbook_1", nil
}

func (f *fakeDB) ListPlaybooks(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (f *fakeDB) GetPlaybook(ctx context.Context, playbookID string) ([]byte, error) {
	return []byte(`{"playbook_id":"playbook_1","tenant_id":"t"}`), nil
}

func (f *fakeDB) CreateRunbook(ctx context.Context, payload []byte) (string, error) {
	return "runbook_1", nil
}

func (f *fakeDB) ListRunbooks(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (f *fakeDB) GetRunbook(ctx context.Context, runbookID string) ([]byte, error) {
	return []byte(`{"runbook_id":"runbook_1","tenant_id":"t"}`), nil
}

type fakeContextDB struct {
	fakeDB
}

func (f *fakeContextDB) InsertAuditEvent(ctx context.Context, payload []byte) (string, error) {
	return "audit_1", nil
}

func (f *fakeContextDB) UpsertContextNode(ctx context.Context, node ctxmodel.Node) error {
	return nil
}

func (f *fakeContextDB) UpsertContextEdge(ctx context.Context, edge ctxmodel.Edge) error {
	return nil
}

func (f *fakeContextDB) GetServiceGraph(ctx context.Context, service string) ([]byte, error) {
	return []byte(`{"nodes":[],"edges":[]}`), nil
}

type errorDB struct{}

func (e errorDB) CreatePlan(ctx context.Context, planJSON []byte) (string, error) { return "", errTest }
func (e errorDB) GetPlan(ctx context.Context, planID string) ([]byte, error)      { return nil, errTest }
func (e errorDB) CreateExecution(ctx context.Context, planID string) (string, error) {
	return "", errTest
}
func (e errorDB) CreateApproval(ctx context.Context, planID string, payload []byte) (string, error) {
	return "", errTest
}
func (e errorDB) GetExecution(ctx context.Context, execID string) ([]byte, error) {
	return nil, errTest
}
func (e errorDB) UpdateApprovalStatusByPlan(ctx context.Context, planID, status string) error {
	return errTest
}
func (e errorDB) GetApprovalStatus(ctx context.Context, planID string) (string, error) {
	return "approved", nil
}

func (e errorDB) ListAuditEvents(ctx context.Context, filter db.AuditFilter) ([]byte, error) {
	return nil, errTest
}

func (e errorDB) CreateRunbook(ctx context.Context, payload []byte) (string, error) {
	return "", errTest
}

func (e errorDB) ListRunbooks(ctx context.Context) ([]byte, error) {
	return nil, errTest
}

func (e errorDB) GetRunbook(ctx context.Context, runbookID string) ([]byte, error) {
	return nil, errTest
}

func (e errorDB) CreateSchedule(ctx context.Context, payload []byte) (string, error) {
	return "", errTest
}

func (e errorDB) ListSchedules(ctx context.Context) ([]byte, error) {
	return nil, errTest
}

func (e errorDB) ListContextServices(ctx context.Context) ([]byte, error) {
	return nil, errTest
}

func (e errorDB) CreatePlaybook(ctx context.Context, payload []byte) (string, error) {
	return "", errTest
}

func (e errorDB) ListPlaybooks(ctx context.Context) ([]byte, error) {
	return nil, errTest
}

func (e errorDB) GetPlaybook(ctx context.Context, playbookID string) ([]byte, error) {
	return nil, errTest
}

type executionErrorDB struct {
	fakeDB
}

func (e *executionErrorDB) CreateExecution(ctx context.Context, planID string) (string, error) {
	return "", errTest
}

type approvalTrackingDB struct {
	approvalCalled bool
}

func (a *approvalTrackingDB) CreatePlan(ctx context.Context, planJSON []byte) (string, error) {
	return "plan_1", nil
}
func (a *approvalTrackingDB) GetPlan(ctx context.Context, planID string) ([]byte, error) {
	return []byte(`{}`), nil
}
func (a *approvalTrackingDB) CreateExecution(ctx context.Context, planID string) (string, error) {
	return "exec_1", nil
}
func (a *approvalTrackingDB) GetExecution(ctx context.Context, execID string) ([]byte, error) {
	return []byte(`{}`), nil
}
func (a *approvalTrackingDB) CreateApproval(ctx context.Context, planID string, payload []byte) (string, error) {
	a.approvalCalled = true
	return "approval_1", nil
}
func (a *approvalTrackingDB) UpdateApprovalStatusByPlan(ctx context.Context, planID, status string) error {
	return nil
}

func (a *approvalTrackingDB) ListAuditEvents(ctx context.Context, filter db.AuditFilter) ([]byte, error) {
	return []byte("[]"), nil
}

func (a *approvalTrackingDB) ListContextServices(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (a *approvalTrackingDB) CreateSchedule(ctx context.Context, payload []byte) (string, error) {
	return "schedule_1", nil
}

func (a *approvalTrackingDB) ListSchedules(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (a *approvalTrackingDB) CreatePlaybook(ctx context.Context, payload []byte) (string, error) {
	return "playbook_1", nil
}

func (a *approvalTrackingDB) ListPlaybooks(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (a *approvalTrackingDB) GetPlaybook(ctx context.Context, playbookID string) ([]byte, error) {
	return []byte(`{"playbook_id":"playbook_1","tenant_id":"t"}`), nil
}

func (a *approvalTrackingDB) CreateRunbook(ctx context.Context, payload []byte) (string, error) {
	return "runbook_1", nil
}

func (a *approvalTrackingDB) ListRunbooks(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (a *approvalTrackingDB) GetRunbook(ctx context.Context, runbookID string) ([]byte, error) {
	return []byte(`{"runbook_id":"runbook_1","tenant_id":"t"}`), nil
}

type fakePlanner struct {
	called   bool
	intent   string
	context  any
	evidence any
	resp     string
	err      error
}

func (f *fakePlanner) Plan(intent string, context any, evidence any) (string, error) {
	f.called = true
	f.intent = intent
	f.context = context
	f.evidence = evidence
	return f.resp, f.err
}

type planDetailsDB struct {
	fakeDB
	stepsErr     error
	approvalsErr error
	steps        []byte
	approvals    []byte
}

func (p *planDetailsDB) ListPlanSteps(ctx context.Context, planID string) ([]byte, error) {
	if p.stepsErr != nil {
		return nil, p.stepsErr
	}
	return p.steps, nil
}

func (p *planDetailsDB) ListApprovalsByPlan(ctx context.Context, planID string) ([]byte, error) {
	if p.approvalsErr != nil {
		return nil, p.approvalsErr
	}
	return p.approvals, nil
}

type missingExecDB struct{}

func (m missingExecDB) CreatePlan(ctx context.Context, planJSON []byte) (string, error) {
	return "plan_1", nil
}
func (m missingExecDB) GetPlan(ctx context.Context, planID string) ([]byte, error) {
	return nil, nil
}
func (m missingExecDB) CreateExecution(ctx context.Context, planID string) (string, error) {
	return "exec_1", nil
}
func (m missingExecDB) CreateApproval(ctx context.Context, planID string, payload []byte) (string, error) {
	return "approval_1", nil
}
func (m missingExecDB) GetExecution(ctx context.Context, execID string) ([]byte, error) {
	return nil, nil
}
func (m missingExecDB) UpdateApprovalStatusByPlan(ctx context.Context, planID, status string) error {
	return nil
}

func (m missingExecDB) ListAuditEvents(ctx context.Context, filter db.AuditFilter) ([]byte, error) {
	return []byte("[]"), nil
}

func (m missingExecDB) ListContextServices(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (m missingExecDB) ListContextSnapshots(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (m missingExecDB) GetContextSnapshot(ctx context.Context, snapshotID string) ([]byte, error) {
	return []byte(`{"snapshot_id":"snap_1","labels":{"tenant_id":"t"},"nodes":[],"edges":[]}`), nil
}

func (m missingExecDB) CreateSchedule(ctx context.Context, payload []byte) (string, error) {
	return "schedule_1", nil
}

func (m missingExecDB) ListSchedules(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (m missingExecDB) CreatePlaybook(ctx context.Context, payload []byte) (string, error) {
	return "playbook_1", nil
}

func (m missingExecDB) ListPlaybooks(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (m missingExecDB) GetPlaybook(ctx context.Context, playbookID string) ([]byte, error) {
	return []byte(`{"playbook_id":"playbook_1","tenant_id":"t"}`), nil
}

func (m missingExecDB) CreateRunbook(ctx context.Context, payload []byte) (string, error) {
	return "runbook_1", nil
}

func (m missingExecDB) ListRunbooks(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (m missingExecDB) GetRunbook(ctx context.Context, runbookID string) ([]byte, error) {
	return []byte(`{"runbook_id":"runbook_1","tenant_id":"t"}`), nil
}

func (m missingExecDB) CreateWorkflowCatalog(ctx context.Context, payload []byte) (string, error) {
	return "workflow_1", nil
}

func (m missingExecDB) ListWorkflowCatalog(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (m missingExecDB) CreateSession(ctx context.Context, payload []byte) (string, error) {
	return "session_1", nil
}

func (m missingExecDB) ListSessions(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (m missingExecDB) GetSession(ctx context.Context, sessionID string) ([]byte, error) {
	return []byte(`{"session_id":"session_1","tenant_id":"t1"}`), nil
}

func (m missingExecDB) UpdateSession(ctx context.Context, sessionID string, payload []byte) error {
	return nil
}

func (m missingExecDB) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (m missingExecDB) AddSessionMember(ctx context.Context, sessionID string, payload []byte) error {
	return nil
}

func (m missingExecDB) ListSessionMembers(ctx context.Context, sessionID string) ([]byte, error) {
	return []byte("[]"), nil
}

func (m missingExecDB) IsSessionMember(ctx context.Context, sessionID, memberID string) (bool, error) {
	return true, nil
}

var errTest = errors.New("db error")

type approvalErrorDB struct {
	fakeDB
}

func (a *approvalErrorDB) CreateApproval(ctx context.Context, planID string, payload []byte) (string, error) {
	return "", errTest
}

type updateErrorDB struct {
	fakeDB
}

func (u *updateErrorDB) UpdateApprovalStatusByPlan(ctx context.Context, planID, status string) error {
	return errTest
}

type auditTrackingDB struct {
	fakeDB
	filter db.AuditFilter
}

func (a *auditTrackingDB) ListAuditEvents(ctx context.Context, filter db.AuditFilter) ([]byte, error) {
	a.filter = filter
	return []byte("[]"), nil
}

type fakeApprovalCreator struct {
	called bool
	planID string
	err    error
}

func (f *fakeApprovalCreator) CreateApprovalIssue(ctx context.Context, planID string) (string, error) {
	f.called = true
	f.planID = planID
	if f.err != nil {
		return "", f.err
	}
	return "issue_1", nil
}

type fakeAuditWriter struct {
	count int
	last  map[string]any
	err   error
}

func (f *fakeAuditWriter) InsertAuditEvent(ctx context.Context, payload []byte) (string, error) {
	f.count++
	_ = json.Unmarshal(payload, &f.last)
	return "audit_1", f.err
}

type fakeContextManager struct {
	refreshCalled bool
	refreshErr    error
	ingestNodes   []ctxmodel.Node
	ingestEdges   []ctxmodel.Edge
	ingestErr     error
	graph         ctxmodel.ServiceGraph
	graphErr      error
	lastService   string
}

func (f *fakeContextManager) RefreshContext(ctx context.Context) error {
	f.refreshCalled = true
	return f.refreshErr
}

func (f *fakeContextManager) IngestSnapshot(ctx context.Context, nodes []ctxmodel.Node, edges []ctxmodel.Edge) error {
	f.ingestNodes = nodes
	f.ingestEdges = edges
	return f.ingestErr
}

func (f *fakeContextManager) GetServiceGraph(ctx context.Context, service string) (ctxmodel.ServiceGraph, error) {
	f.lastService = service
	return f.graph, f.graphErr
}

type denyChecker struct{}

func (denyChecker) Evaluate(input policy.PolicyInput) (policy.PolicyDecision, error) {
	return policy.PolicyDecision{Decision: "deny"}, nil
}

type requireApprovalChecker struct{}

func (requireApprovalChecker) Evaluate(input policy.PolicyInput) (policy.PolicyDecision, error) {
	return policy.PolicyDecision{Decision: "require_approval"}, nil
}

type errorChecker struct{}

func (errorChecker) Evaluate(input policy.PolicyInput) (policy.PolicyDecision, error) {
	return policy.PolicyDecision{}, errTest
}

type unknownDecisionChecker struct{}

func (unknownDecisionChecker) Evaluate(input policy.PolicyInput) (policy.PolicyDecision, error) {
	return policy.PolicyDecision{Decision: "weird"}, nil
}

type allowChecker struct{}

func (allowChecker) Evaluate(input policy.PolicyInput) (policy.PolicyDecision, error) {
	return policy.PolicyDecision{Decision: "allow"}, nil
}

type auditDB struct {
	fakeDB
}

func (a *auditDB) InsertAuditEvent(ctx context.Context, payload []byte) (string, error) {
	return "audit_1", nil
}

type errReadCloser struct{}

func (errReadCloser) Read(_ []byte) (int, error) {
	return 0, errors.New("read error")
}

func (errReadCloser) Close() error {
	return nil
}

func validContext() ContextRef {
	return ContextRef{
		TenantID:      "t",
		Environment:   "dev",
		ClusterID:     "cluster-1",
		Namespace:     "default",
		AWSAccountID:  "123456789012",
		Region:        "us-east-1",
		ArgoCDProject: "proj",
		GrafanaOrgID:  "1",
	}
}

func mustPlanJSON(t *testing.T, plan map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return data
}

func TestHandlePlansCreateFlow(t *testing.T) {
	db := &fakeDB{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlansReadRisk(t *testing.T) {
	db := &fakeDB{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "show status"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var plan map[string]any
	if err := json.Unmarshal(db.lastPlan, &plan); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if plan["risk_level"] != "read" {
		t.Fatalf("risk: %#v", plan["risk_level"])
	}
}

func TestHandlePlansPlannerSuccess(t *testing.T) {
	db := &fakeDB{}
	planner := &fakePlanner{resp: `{"steps":[{"action":"deploy","tool":"helm","input":{"release":"app"}}]}`}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Planner: planner, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !planner.called {
		t.Fatalf("expected planner call")
	}
	var plan map[string]any
	if err := json.Unmarshal(db.lastPlan, &plan); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if plan["plan_text"] == nil || plan["intent"] != "deploy" {
		t.Fatalf("plan: %#v", plan)
	}
	if steps, ok := plan["steps"].([]any); !ok || len(steps) != 1 {
		t.Fatalf("steps: %#v", plan["steps"])
	}
}

func TestHandlePlansPlannerError(t *testing.T) {
	db := &fakeDB{}
	planner := &fakePlanner{err: errors.New("boom")}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Planner: planner, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status: %d", w.Code)
	}
	if db.lastPlan != nil {
		t.Fatalf("unexpected plan")
	}
}

func TestHandlePlansAuditAllow(t *testing.T) {
	db := &fakeDB{}
	audit := &fakeAuditWriter{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Audit: audit, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if audit.count != 1 {
		t.Fatalf("audit count: %d", audit.count)
	}
	if audit.last["action"] != "plan.create" || audit.last["decision"] != "allow" {
		t.Fatalf("audit: %#v", audit.last)
	}
}

func TestHandlePlansAuditDeny(t *testing.T) {
	db := &fakeDB{}
	audit := &fakeAuditWriter{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: denyChecker{}}, Audit: audit}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
	if audit.count != 1 {
		t.Fatalf("audit count: %d", audit.count)
	}
	if audit.last["decision"] != "deny" {
		t.Fatalf("audit: %#v", audit.last)
	}
}

func TestHandlePlansPolicyError(t *testing.T) {
	db := &fakeDB{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: errorChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlansPolicyUnknownDecision(t *testing.T) {
	db := &fakeDB{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: unknownDecisionChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlansAutoApproveLow(t *testing.T) {
	db := &fakeDB{}
	approvals := &fakeApprovalCreator{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Approvals: approvals, AutoApproveLow: true, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if approvals.called {
		t.Fatalf("unexpected external approval")
	}
	if db.updatePlan != "plan_1" || db.updateStatus != "approved" {
		t.Fatalf("auto approve: %s %s", db.updatePlan, db.updateStatus)
	}
}

func TestHandlePlansAutoApproveUpdateError(t *testing.T) {
	db := &updateErrorDB{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, AutoApproveLow: true, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlansAutoApproveCreateError(t *testing.T) {
	db := &approvalErrorDB{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, AutoApproveLow: true, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlansPolicyRequireApprovalOverridesAuto(t *testing.T) {
	db := &fakeDB{}
	approvals := &fakeApprovalCreator{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{
		Mux:            http.NewServeMux(),
		DB:             db,
		Approvals:      approvals,
		AutoApproveLow: true,
		Policy:         &policy.Evaluator{Checker: requireApprovalChecker{}},
	}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !approvals.called {
		t.Fatalf("expected external approval")
	}
	if db.updateStatus != "" {
		t.Fatalf("unexpected auto approve")
	}
}

func TestHandlePlansPolicyRequireApprovalRead(t *testing.T) {
	db := &fakeDB{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "show status"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: requireApprovalChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestNewServerRegistersRoutes(t *testing.T) {
	srv := NewServer(&fakeDB{}, nil)
	if srv == nil || srv.Mux == nil {
		t.Fatalf("expected server")
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/plans", nil)
	_, pattern := srv.Mux.Handler(req)
	if pattern == "" {
		t.Fatalf("expected route")
	}
}

func TestNewServerSetsAuditWriter(t *testing.T) {
	srv := NewServer(&auditDB{}, nil)
	if srv.Audit == nil {
		t.Fatalf("expected audit writer")
	}
}

func TestAuditEventNoWriter(t *testing.T) {
	srv := &Server{}
	srv.auditEvent(context.Background(), "plan.create", "allow", ContextRef{}, "")
}

func TestAuditEventMarshalError(t *testing.T) {
	audit := &fakeAuditWriter{}
	srv := &Server{Audit: audit}
	srv.auditEvent(context.Background(), "plan.create", "allow", make(chan int), "")
	if audit.count != 0 {
		t.Fatalf("unexpected audit call")
	}
}

func TestAuditEventWriterError(t *testing.T) {
	audit := &fakeAuditWriter{err: errors.New("boom")}
	srv := &Server{Audit: audit}
	srv.auditEvent(context.Background(), "plan.create", "allow", ContextRef{}, "")
	if audit.count != 1 {
		t.Fatalf("audit count: %d", audit.count)
	}
}

func TestHandlePlansReadIntentNoApproval(t *testing.T) {
	db := &approvalTrackingDB{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "status"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if db.approvalCalled {
		t.Fatalf("unexpected approval")
	}
}

func TestHandlePlansApprovalError(t *testing.T) {
	db := &fakeDB{}
	approvals := &fakeApprovalCreator{err: errors.New("fail")}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()

	srv := &Server{Mux: http.NewServeMux(), DB: db, Approvals: approvals, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status: %d", w.Code)
	}
	if !approvals.called || approvals.planID == "" {
		t.Fatalf("approvals: %#v", approvals)
	}
}

func TestHandlePlansCreateInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader([]byte("{")))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	srv := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlansDBUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	srv := &Server{Mux: http.NewServeMux(), DB: nil}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlansMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/plans", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	srv := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(srv.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlansPolicyDeny(t *testing.T) {
	db := &fakeDB{}
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: denyChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlansDBError(t *testing.T) {
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: errorDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlansEncodeError(t *testing.T) {
	ctx := validContext()
	body, _ := json.Marshal(PlanCreateRequest{Summary: "s", Trigger: "manual", Context: ctx, Intent: "deploy", Constraints: map[string]any{"key": "value"}})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	oldMarshal := marshalJSON
	marshalJSON = func(v any) ([]byte, error) { return nil, errTest }
	defer func() { marshalJSON = oldMarshal }()
	AuthMiddleware(http.HandlerFunc(server.handlePlans)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteFlow(t *testing.T) {
	plan := map[string]any{"plan_id": "plan_1", "risk_level": "low", "context": validContext()}
	db := &fakeDB{planID: "plan_1", lastPlan: mustPlanJSON(t, plan)}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteInvalidJSON(t *testing.T) {
	db := &fakeDB{planID: "plan_1", lastPlan: []byte(`{"plan_id":"plan_1"}`)}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", bytes.NewReader([]byte("{")))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteMissingPlanID(t *testing.T) {
	db := &fakeDB{planID: "plan_1", lastPlan: []byte(`{"plan_id":"plan_1"}`)}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecutePlanNotFound(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecutePlanDecodeError(t *testing.T) {
	db := &fakeDB{planID: "plan_1", lastPlan: []byte("{")}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteReadNoApproval(t *testing.T) {
	db := &fakeDB{
		planID:         "plan_1",
		lastPlan:       mustPlanJSON(t, map[string]any{"plan_id": "plan_1", "risk_level": "read", "context": validContext()}),
		approvalStatus: "denied",
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteRiskFromIntent(t *testing.T) {
	db := &fakeDB{
		planID:   "plan_1",
		lastPlan: mustPlanJSON(t, map[string]any{"plan_id": "plan_1", "intent": "status check", "context": validContext()}),
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteApprovalError(t *testing.T) {
	plan := map[string]any{"plan_id": "plan_1", "risk_level": "low", "context": validContext()}
	db := &fakeDB{
		planID:      "plan_1",
		lastPlan:    mustPlanJSON(t, plan),
		approvalErr: errTest,
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteConstraintViolation(t *testing.T) {
	plan := map[string]any{
		"plan_id":    "plan_1",
		"risk_level": "low",
		"context":    validContext(),
		"constraints": map[string]any{
			"max_targets": 1,
		},
		"steps": []map[string]any{
			{
				"action": "scale",
				"tool":   "kubectl",
				"input": map[string]any{
					"targets": []string{"a", "b"},
				},
			},
		},
	}
	db := &fakeDB{
		planID:   "plan_1",
		lastPlan: mustPlanJSON(t, plan),
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteConstraintFromDetails(t *testing.T) {
	plan := map[string]any{
		"plan_id":    "plan_1",
		"risk_level": "low",
		"context":    validContext(),
		"constraints": map[string]any{
			"max_targets": 1,
		},
	}
	db := &planDetailsDB{
		fakeDB: fakeDB{
			planID:   "plan_1",
			lastPlan: mustPlanJSON(t, plan),
		},
		steps: []byte(`[{"action":"scale","tool":"kubectl","input":{"targets":["a","b"]}}]`),
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanDiffOK(t *testing.T) {
	plan := map[string]any{
		"plan_id":    "plan_1",
		"risk_level": "low",
		"context":    validContext(),
		"steps": []map[string]any{
			{"action": "scale", "tool": "kubectl", "input": map[string]any{"resource": "deploy/app", "replicas": 2}},
		},
	}
	db := &fakeDB{planID: "plan_1", lastPlan: mustPlanJSON(t, plan)}
	req := httptest.NewRequest(http.MethodGet, "/v1/plans/plan_1/diff", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "changes") {
		t.Fatalf("body: %s", w.Body.String())
	}
}

func TestHandlePlanRiskOK(t *testing.T) {
	plan := map[string]any{
		"plan_id":    "plan_1",
		"risk_level": "low",
		"context":    validContext(),
	}
	db := &fakeDB{planID: "plan_1", lastPlan: mustPlanJSON(t, plan)}
	req := httptest.NewRequest(http.MethodGet, "/v1/plans/plan_1/risk", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "blast_radius") {
		t.Fatalf("body: %s", w.Body.String())
	}
}

func TestHandlePlanExecuteCreateExecutionError(t *testing.T) {
	db := &executionErrorDB{
		fakeDB: fakeDB{
			planID:   "plan_1",
			lastPlan: mustPlanJSON(t, map[string]any{"plan_id": "plan_1", "risk_level": "read", "context": validContext()}),
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteDBUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: nil}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleExecutionGet(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/exec_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleExecutionByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanGetOK(t *testing.T) {
	db := &fakeDB{planID: "plan_1", lastPlan: mustPlanJSON(t, map[string]any{"plan_id": "plan_1", "context": validContext()})}
	req := httptest.NewRequest(http.MethodGet, "/v1/plans/plan_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanGetWithDetails(t *testing.T) {
	db := &planDetailsDB{
		fakeDB:    fakeDB{planID: "plan_1", lastPlan: mustPlanJSON(t, map[string]any{"plan_id": "plan_1", "context": validContext()})},
		steps:     []byte(`[{"step_id":"step_1"}]`),
		approvals: []byte(`[{"approval_id":"a1"}]`),
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/plans/plan_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var plan map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if plan["steps"] == nil || plan["approvals"] == nil {
		t.Fatalf("plan: %#v", plan)
	}
}

func TestHandlePlanGetDetailsStepError(t *testing.T) {
	db := &planDetailsDB{
		fakeDB:   fakeDB{planID: "plan_1", lastPlan: mustPlanJSON(t, map[string]any{"plan_id": "plan_1", "context": validContext()})},
		stepsErr: errors.New("boom"),
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/plans/plan_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanGetDetailsApprovalError(t *testing.T) {
	db := &planDetailsDB{
		fakeDB:       fakeDB{planID: "plan_1", lastPlan: mustPlanJSON(t, map[string]any{"plan_id": "plan_1", "context": validContext()})},
		steps:        []byte(`[]`),
		approvalsErr: errors.New("boom"),
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/plans/plan_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanGetDecodeError(t *testing.T) {
	db := &fakeDB{planID: "plan_1", lastPlan: []byte("{")}
	req := httptest.NewRequest(http.MethodGet, "/v1/plans/plan_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleExecutionNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/missing", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: missingExecDB{}}
	AuthMiddleware(http.HandlerFunc(server.handleExecutionByID)).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanNotFound(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodGet, "/v1/plans/missing", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanGetDBError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/plans/plan_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: errorDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanGetDBUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/plans/plan_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: nil}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/v1/plans/plan_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecutePolicyDeny(t *testing.T) {
	plan := map[string]any{"plan_id": "plan_1", "risk_level": "low", "context": validContext()}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{planID: "plan_1", lastPlan: mustPlanJSON(t, plan)}, Policy: &policy.Evaluator{Checker: denyChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecutePolicyError(t *testing.T) {
	plan := map[string]any{"plan_id": "plan_1", "risk_level": "low", "context": validContext()}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{planID: "plan_1", lastPlan: mustPlanJSON(t, plan)}, Policy: &policy.Evaluator{Checker: errorChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecutePolicyUnknownDecision(t *testing.T) {
	plan := map[string]any{"plan_id": "plan_1", "risk_level": "low", "context": validContext()}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{planID: "plan_1", lastPlan: mustPlanJSON(t, plan)}, Policy: &policy.Evaluator{Checker: unknownDecisionChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecutePolicyRequireApprovalRead(t *testing.T) {
	db := &fakeDB{planID: "plan_1", lastPlan: mustPlanJSON(t, map[string]any{"plan_id": "plan_1", "risk_level": "read", "context": validContext()})}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: requireApprovalChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecutePolicyRequireApprovalWrite(t *testing.T) {
	plan := map[string]any{"plan_id": "plan_1", "risk_level": "low", "context": validContext()}
	db := &fakeDB{
		planID:         "plan_1",
		lastPlan:       mustPlanJSON(t, plan),
		approvalStatus: "approved",
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: requireApprovalChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteDBError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: errorDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteApprovalDenied(t *testing.T) {
	plan := map[string]any{"plan_id": "plan_1", "risk_level": "low", "context": validContext()}
	db := &fakeDB{
		planID:         "plan_1",
		lastPlan:       mustPlanJSON(t, plan),
		approvalStatus: "denied",
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

type tokenApprovalDB struct {
	fakeDB
	token       string
	tokenStatus string
	tokenErr    error
}

func (t *tokenApprovalDB) GetApprovalStatusByToken(ctx context.Context, planID, approvalID string) (string, error) {
	if t.tokenErr != nil {
		return "", t.tokenErr
	}
	if approvalID != t.token {
		return "", errTest
	}
	return t.tokenStatus, nil
}

func TestHandlePlanExecuteApprovalToken(t *testing.T) {
	plan := map[string]any{"plan_id": "plan_1", "risk_level": "low", "context": validContext()}
	db := &tokenApprovalDB{
		fakeDB: fakeDB{
			planID:         "plan_1",
			lastPlan:       mustPlanJSON(t, plan),
			approvalStatus: "denied",
		},
		token:       "token",
		tokenStatus: "approved",
	}
	body, _ := json.Marshal(PlanExecuteRequest{ApprovalToken: "token"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteApprovalTokenError(t *testing.T) {
	plan := map[string]any{"plan_id": "plan_1", "risk_level": "low", "context": validContext()}
	db := &tokenApprovalDB{
		fakeDB: fakeDB{
			planID:   "plan_1",
			lastPlan: mustPlanJSON(t, plan),
		},
		token:    "token",
		tokenErr: errTest,
	}
	body, _ := json.Marshal(PlanExecuteRequest{ApprovalToken: "token"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlanExecuteApprovalTokenUnsupported(t *testing.T) {
	plan := map[string]any{"plan_id": "plan_1", "risk_level": "low", "context": validContext()}
	db := &fakeDB{planID: "plan_1", lastPlan: mustPlanJSON(t, plan)}
	body, _ := json.Marshal(PlanExecuteRequest{ApprovalToken: "token"})
	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestApprovalStatusMissingPlanID(t *testing.T) {
	srv := &Server{DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	if _, err := srv.approvalStatus(context.Background(), "", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestApprovalStatusMissingDB(t *testing.T) {
	srv := &Server{}
	if _, err := srv.approvalStatus(context.Background(), "plan", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestApprovalStatusUnavailable(t *testing.T) {
	srv := &Server{DB: &approvalTrackingDB{}}
	if _, err := srv.approvalStatus(context.Background(), "plan", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestHandleApprovalsMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/approvals", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleApprovalsDBUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: nil}
	AuthMiddleware(http.HandlerFunc(server.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleApprovalsBadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals", bytes.NewReader([]byte("{")))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleApprovalsOK(t *testing.T) {
	body, _ := json.Marshal(ApprovalCreateRequest{PlanID: "plan"})
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleApprovalsUpdateStatus(t *testing.T) {
	db := &fakeDB{}
	body, _ := json.Marshal(ApprovalCreateRequest{PlanID: "plan_2", Status: "approved"})
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if db.updatePlan != "plan_2" || db.updateStatus != "approved" {
		t.Fatalf("update: %s %s", db.updatePlan, db.updateStatus)
	}
}

func TestHandleApprovalsPendingWithIssue(t *testing.T) {
	db := &fakeDB{}
	approvals := &fakeApprovalCreator{}
	body, _ := json.Marshal(ApprovalCreateRequest{PlanID: "plan_3", Status: "pending"})
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Approvals: approvals, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !approvals.called || approvals.planID != "plan_3" {
		t.Fatalf("approvals: %#v", approvals)
	}
}

func TestHandleApprovalsApprovalError(t *testing.T) {
	db := &fakeDB{}
	approvals := &fakeApprovalCreator{err: errors.New("fail")}
	body, _ := json.Marshal(ApprovalCreateRequest{PlanID: "plan_4"})
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Approvals: approvals, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleApprovalsPolicyDeny(t *testing.T) {
	body, _ := json.Marshal(ApprovalCreateRequest{PlanID: "plan", Status: "approved"})
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: denyChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleApprovalsDBError(t *testing.T) {
	body, _ := json.Marshal(ApprovalCreateRequest{PlanID: "plan", Status: "approved"})
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: errorDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleApprovalsPendingDBError(t *testing.T) {
	body, _ := json.Marshal(ApprovalCreateRequest{PlanID: "plan"})
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: errorDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleApprovalsInvalidStatus(t *testing.T) {
	body, _ := json.Marshal(ApprovalCreateRequest{PlanID: "plan", Status: "nope"})
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleApprovalsMissingPlanID(t *testing.T) {
	body, _ := json.Marshal(ApprovalCreateRequest{PlanID: "", Status: "approved"})
	req := httptest.NewRequest(http.MethodPost, "/v1/approvals", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleApprovals)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleHookOK(t *testing.T) {
	body := []byte(`{"event":"x"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/argocd", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	audit := &fakeAuditWriter{}
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Audit: audit, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp HookAck
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.EventID == "" {
		t.Fatalf("missing event id")
	}
	if audit.count != 2 {
		t.Fatalf("audit count: %d", audit.count)
	}
	if audit.last["action"] != "plan.create" {
		t.Fatalf("audit: %#v", audit.last)
	}
}

func TestHandleHookMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/hooks/argocd", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleHookInvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/argocd", nil)
	req.Body = errReadCloser{}
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleHookInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/alertmanager", bytes.NewReader([]byte("{")))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleHookNoDB(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/git", bytes.NewReader(nil))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: nil}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp HookAck
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.EventID != "" {
		t.Fatalf("expected empty event id")
	}
}

func TestHandleHookCreatesPlan(t *testing.T) {
	db := &fakeDB{}
	body := []byte(`{"alerts":[{"labels":{"alertname":"HighError","namespace":"ns","cluster":"c1","env":"prod","tenant_id":"t"}}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/alertmanager", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var plan map[string]any
	if err := json.Unmarshal(db.lastPlan, &plan); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if plan["summary"] != "Alertmanager: HighError" {
		t.Fatalf("summary: %#v", plan["summary"])
	}
	if plan["trigger"] != "webhook" {
		t.Fatalf("trigger: %#v", plan["trigger"])
	}
	ctx, _ := plan["context"].(map[string]any)
	if ctx["namespace"] != "ns" || ctx["cluster_id"] != "c1" || ctx["environment"] != "prod" || ctx["tenant_id"] != "t" {
		t.Fatalf("context: %#v", ctx)
	}
}

func TestHandleHookPlannerSuccess(t *testing.T) {
	db := &fakeDB{}
	planner := &fakePlanner{resp: `{"steps":[{"action":"deploy","tool":"helm","input":{"release":"app"}}]}`}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/git", bytes.NewReader([]byte(`{"repo":"x"}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Planner: planner, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !planner.called {
		t.Fatalf("planner not called")
	}
	if planner.intent != "Git webhook" {
		t.Fatalf("intent: %s", planner.intent)
	}
	contextMap, ok := planner.context.(map[string]any)
	if !ok {
		t.Fatalf("context: %#v", planner.context)
	}
	if contextMap["summary"] != "Git webhook" {
		t.Fatalf("summary: %#v", contextMap["summary"])
	}
	if contextMap["source"] != "git" {
		t.Fatalf("source: %#v", contextMap["source"])
	}
	if contextMap["trigger"] != "webhook" {
		t.Fatalf("trigger: %#v", contextMap["trigger"])
	}
	var plan map[string]any
	if err := json.Unmarshal(db.lastPlan, &plan); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if plan["plan_text"] != planner.resp {
		t.Fatalf("plan_text: %#v", plan["plan_text"])
	}
	if plan["intent"] != "Git webhook" {
		t.Fatalf("intent: %#v", plan["intent"])
	}
	steps, ok := plan["steps"].([]any)
	if !ok || len(steps) != 1 {
		t.Fatalf("steps: %#v", plan["steps"])
	}
	step, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("step: %#v", steps[0])
	}
	if step["action"] != "deploy" || step["tool"] != "helm" {
		t.Fatalf("step fields: %#v", step)
	}
}

func TestHandleHookPlannerError(t *testing.T) {
	db := &fakeDB{}
	planner := &fakePlanner{err: errors.New("boom")}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/git", bytes.NewReader([]byte(`{"repo":"x"}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Planner: planner, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status: %d", w.Code)
	}
	if !planner.called {
		t.Fatalf("planner not called")
	}
	if db.planID != "" || len(db.lastPlan) != 0 {
		t.Fatalf("plan created on error")
	}
}

func TestHandleHookEmptyBodyWithDB(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/git", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var plan map[string]any
	if err := json.Unmarshal(db.lastPlan, &plan); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if plan["summary"] != "Git webhook" {
		t.Fatalf("summary: %#v", plan["summary"])
	}
}

func TestHandleHookAutoApproveLow(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/deploy", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, AutoApproveLow: true, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if db.updateStatus != "approved" {
		t.Fatalf("approval status: %s", db.updateStatus)
	}
}

func TestHandleHookAutoApproveCreateApprovalError(t *testing.T) {
	db := &approvalErrorDB{}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/deploy", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, AutoApproveLow: true, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleHookAutoApproveUpdateError(t *testing.T) {
	db := &updateErrorDB{}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/deploy", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, AutoApproveLow: true, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleHookManualApprovalCreateError(t *testing.T) {
	db := &approvalErrorDB{}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/deploy", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleHookPolicyRequireApprovalWrite(t *testing.T) {
	db := &fakeDB{}
	approvals := &fakeApprovalCreator{}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/deploy", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{
		Mux:       http.NewServeMux(),
		DB:        db,
		Approvals: approvals,
		Policy:    &policy.Evaluator{Checker: requireApprovalChecker{}},
	}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !approvals.called {
		t.Fatalf("expected external approval")
	}
}

func TestHandleHookPolicyRequireApprovalRead(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/git", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: requireApprovalChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleHookPolicyError(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/git", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: errorChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleHookPolicyErrorWrite(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/deploy", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: errorChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleHookPolicyUnknownDecision(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/git", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: unknownDecisionChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleHookPolicyDeny(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/git", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	audit := &fakeAuditWriter{}
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: denyChecker{}}, Audit: audit}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
	if audit.count != 2 {
		t.Fatalf("audit count: %d", audit.count)
	}
	if audit.last["decision"] != "deny" {
		t.Fatalf("audit: %#v", audit.last)
	}
}

func TestHandleHookDBError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/git", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: errorDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleHookEncodeError(t *testing.T) {
	oldMarshal := marshalJSON
	marshalJSON = func(_ any) ([]byte, error) { return nil, errors.New("boom") }
	defer func() { marshalJSON = oldMarshal }()

	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/git", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHookSummaryVariants(t *testing.T) {
	if got := hookSummary("argocd", map[string]any{}); got != "Argo CD webhook" {
		t.Fatalf("argocd: %s", got)
	}
	if got := hookSummary("git", map[string]any{}); got != "Git webhook" {
		t.Fatalf("git: %s", got)
	}
	if got := hookSummary("", map[string]any{}); got != "Webhook" {
		t.Fatalf("empty: %s", got)
	}
	if got := hookSummary("other", map[string]any{}); got != "Webhook: other" {
		t.Fatalf("other: %s", got)
	}
	if got := hookSummary("alertmanager", map[string]any{}); got != "Alertmanager webhook" {
		t.Fatalf("alertmanager default: %s", got)
	}
}

func TestContextFromHookContextField(t *testing.T) {
	payload := map[string]any{"context": map[string]any{"namespace": "ns", "cluster_id": "c1", "environment": "dev"}}
	ctx := contextFromHook(payload)
	if ctx.Namespace != "ns" || ctx.ClusterID != "c1" || ctx.Environment != "dev" {
		t.Fatalf("ctx: %#v", ctx)
	}
}

func TestContextFromHookNil(t *testing.T) {
	ctx := contextFromHook(nil)
	if ctx != (ContextRef{}) {
		t.Fatalf("expected empty context")
	}
}

func TestContextFromHookContextInvalid(t *testing.T) {
	payload := map[string]any{
		"context": "bad",
		"alerts":  []any{map[string]any{"labels": map[string]any{"namespace": "ns"}}},
	}
	ctx := contextFromHook(payload)
	if ctx.Namespace != "ns" {
		t.Fatalf("ctx: %#v", ctx)
	}
}

func TestExtractAlertLabelsInvalid(t *testing.T) {
	if labels := extractAlertLabels(map[string]any{"alerts": []any{1}}); labels != nil {
		t.Fatalf("expected nil")
	}
	if labels := extractAlertLabels(map[string]any{}); labels != nil {
		t.Fatalf("expected nil")
	}
}

func TestExtractAlertNameCommonLabels(t *testing.T) {
	payload := map[string]any{"commonLabels": map[string]any{"alertname": "Foo"}}
	if name := extractAlertName(payload); name != "Foo" {
		t.Fatalf("name: %s", name)
	}
}

func TestExtractAlertNameMissing(t *testing.T) {
	payload := map[string]any{"commonLabels": map[string]any{"x": "y"}}
	if name := extractAlertName(payload); name != "" {
		t.Fatalf("name: %s", name)
	}
}

func TestHandleExecutionMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/executions/exec", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleExecutionByID)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleExecutionDBUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/exec", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: nil}
	AuthMiddleware(http.HandlerFunc(server.handleExecutionByID)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleExecutionDBError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/exec", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: errorDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleExecutionByID)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleAuditEvents(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/audit/events", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleAuditEvents)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleAuditEventsQueryParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/audit/events?from=2024-01-01T00:00:00Z&to=2024-01-02T00:00:00Z&actor_id=user&action=deploy&decision=allow", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	db := &auditTrackingDB{}
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleAuditEvents)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if db.filter.ActorID != "user" || db.filter.Action != "deploy" || db.filter.Decision != "allow" {
		t.Fatalf("filter: %#v", db.filter)
	}
}

func TestHandleAuditEventsInvalidQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/audit/events?from=bad", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleAuditEvents)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleAuditEventsInvalidTo(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/audit/events?to=bad", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleAuditEvents)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleAuditEventsDBUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/audit/events", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: nil}
	AuthMiddleware(http.HandlerFunc(server.handleAuditEvents)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleAuditEventsDBError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/audit/events", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: errorDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleAuditEvents)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleAuditEventsMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/audit/events", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleAuditEvents)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextServices(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/context/services", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextServices)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextServicesDBUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/context/services", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: nil}
	AuthMiddleware(http.HandlerFunc(server.handleContextServices)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextServicesDBError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/context/services", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: errorDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextServices)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextServicesMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/context/services", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextServices)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextRefreshIngestOK(t *testing.T) {
	manager := &fakeContextManager{}
	body, _ := json.Marshal(ContextRefreshRequest{
		Service: "svc",
		Nodes:   []ctxmodel.Node{{NodeID: "n1", Kind: "service", Name: "svc"}},
		Edges:   []ctxmodel.Edge{{EdgeID: "e1", FromNodeID: "n1", ToNodeID: "n2", Relation: "calls"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/context/refresh", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: manager, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextRefresh)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if len(manager.ingestNodes) != 1 || len(manager.ingestEdges) != 1 {
		t.Fatalf("ingest nodes=%d edges=%d", len(manager.ingestNodes), len(manager.ingestEdges))
	}
	if manager.refreshCalled {
		t.Fatalf("unexpected refresh")
	}
}

func TestHandleContextRefreshEmptyBody(t *testing.T) {
	manager := &fakeContextManager{}
	req := httptest.NewRequest(http.MethodPost, "/v1/context/refresh", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: manager, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextRefresh)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !manager.refreshCalled {
		t.Fatalf("expected refresh")
	}
}

func TestHandleContextRefreshInvalidJSON(t *testing.T) {
	manager := &fakeContextManager{}
	req := httptest.NewRequest(http.MethodPost, "/v1/context/refresh", bytes.NewBufferString("{"))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: manager, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextRefresh)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextRefreshContextUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/context/refresh", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: nil}
	AuthMiddleware(http.HandlerFunc(server.handleContextRefresh)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextRefreshPolicyDenied(t *testing.T) {
	manager := &fakeContextManager{}
	req := httptest.NewRequest(http.MethodPost, "/v1/context/refresh", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: manager, Policy: &policy.Evaluator{Checker: denyChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextRefresh)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextRefreshMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/context/refresh", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: &fakeContextManager{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextRefresh)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextGraphOK(t *testing.T) {
	manager := &fakeContextManager{
		graph: ctxmodel.ServiceGraph{
			Nodes: []ctxmodel.Node{{NodeID: "n1", Kind: "service", Name: "svc"}},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/graph?service=svc", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: manager, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextGraph)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if manager.lastService != "svc" {
		t.Fatalf("service: %s", manager.lastService)
	}
	if !strings.Contains(w.Body.String(), "\"nodes\"") {
		t.Fatalf("body: %s", w.Body.String())
	}
}

func TestHandleContextGraphMissingService(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/context/graph", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: &fakeContextManager{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextGraph)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextGraphContextUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/context/graph?service=svc", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: nil}
	AuthMiddleware(http.HandlerFunc(server.handleContextGraph)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextGraphError(t *testing.T) {
	manager := &fakeContextManager{graphErr: errors.New("boom")}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/graph?service=svc", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: manager, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextGraph)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextGraphMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/context/graph", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: &fakeContextManager{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextGraph)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextRefreshIngestError(t *testing.T) {
	manager := &fakeContextManager{ingestErr: errors.New("boom")}
	body, _ := json.Marshal(ContextRefreshRequest{
		Nodes: []ctxmodel.Node{{NodeID: "n1", Kind: "service", Name: "svc"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/context/refresh", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: manager, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextRefresh)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextRefreshRefreshError(t *testing.T) {
	manager := &fakeContextManager{refreshErr: errors.New("boom")}
	req := httptest.NewRequest(http.MethodPost, "/v1/context/refresh", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), Context: manager, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleContextRefresh)).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybooksCreateOK(t *testing.T) {
	db := &fakeDB{}
	body, _ := json.Marshal(PlaybookCreateRequest{
		TenantID: "t",
		Name:     "deploy",
		Version:  1,
		Tags:     []string{"sre"},
		Spec:     map[string]any{"steps": []any{}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/playbooks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "playbook_id") {
		t.Fatalf("body: %s", w.Body.String())
	}
}

func TestHandlePlaybooksListOK(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodGet, "/v1/playbooks", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlaybooks)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandlePlaybookByIDOK(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodGet, "/v1/playbooks/playbook_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlaybookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbooksCreateOK(t *testing.T) {
	db := &fakeDB{}
	body, _ := json.Marshal(RunbookCreateRequest{
		TenantID: "t",
		Service:  "api",
		Name:     "deploy",
		Version:  1,
		Tags:     json.RawMessage(`["sre"]`),
		Spec:     json.RawMessage(`{"steps":[]}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/runbooks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "runbook_id") {
		t.Fatalf("body: %s", w.Body.String())
	}
}

func TestHandleRunbooksListOK(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodGet, "/v1/runbooks", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleRunbooks)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleRunbookByIDOK(t *testing.T) {
	db := &fakeDB{}
	req := httptest.NewRequest(http.MethodGet, "/v1/runbooks/runbook_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleRunbookByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

type fakeExecutor struct {
	called bool
}

func (f *fakeExecutor) StartExecution(ctx context.Context, planID, executionID string, ctxRef ContextRef, steps []PlanStep) (string, error) {
	f.called = true
	return "wf_1", nil
}

func TestHandleWorkflowsListOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/workflows", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: &fakeDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handleWorkflows)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWorkflowStartOK(t *testing.T) {
	db := &fakeDB{}
	exec := &fakeExecutor{}
	body, _ := json.Marshal(map[string]any{
		"context": validContext(),
		"input": map[string]any{
			"resource": "deploy/app",
			"replicas": 2,
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/scale_service/start", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}, Executor: exec, AutoApproveLow: true}
	AuthMiddleware(http.HandlerFunc(server.handleWorkflowByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !exec.called {
		t.Fatalf("expected execution")
	}
}

func TestNewServerWiresAuditAndContext(t *testing.T) {
	db := &fakeContextDB{}
	srv := NewServer(db, &policy.Evaluator{Checker: allowChecker{}})
	if srv.Audit == nil {
		t.Fatalf("expected audit writer")
	}
	if srv.Context == nil {
		t.Fatalf("expected context manager")
	}
}

func TestNewServerNilDB(t *testing.T) {
	srv := NewServer(nil, nil)
	if srv.Audit != nil {
		t.Fatalf("unexpected audit writer")
	}
	if srv.Context != nil {
		t.Fatalf("unexpected context manager")
	}
}

func (f *fakeDB) ListContextSnapshots(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (f *fakeDB) GetContextSnapshot(ctx context.Context, snapshotID string) ([]byte, error) {
	return []byte(`{"snapshot_id":"snap_1","labels":{"tenant_id":"t"},"nodes":[],"edges":[]}`), nil
}

func (f *fakeDB) CreateSession(ctx context.Context, payload []byte) (string, error) {
	return "session_1", nil
}

func (f *fakeDB) ListSessions(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (f *fakeDB) GetSession(ctx context.Context, sessionID string) ([]byte, error) {
	return []byte(`{"session_id":"session_1"}`), nil
}

func (f *fakeDB) UpdateSession(ctx context.Context, sessionID string, payload []byte) error {
	return nil
}

func (f *fakeDB) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (f *fakeDB) AddSessionMember(ctx context.Context, sessionID string, payload []byte) error {
	return nil
}

func (f *fakeDB) ListSessionMembers(ctx context.Context, sessionID string) ([]byte, error) {
	return []byte("[]"), nil
}

func (f *fakeDB) CreateWorkflowCatalog(ctx context.Context, payload []byte) (string, error) {
	return "workflow_1", nil
}

func (f *fakeDB) ListWorkflowCatalog(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (e errorDB) ListContextSnapshots(ctx context.Context) ([]byte, error) {
	return nil, errTest
}

func (e errorDB) GetContextSnapshot(ctx context.Context, snapshotID string) ([]byte, error) {
	return nil, errTest
}

func (e errorDB) CreateSession(ctx context.Context, payload []byte) (string, error) {
	return "", errTest
}

func (e errorDB) ListSessions(ctx context.Context) ([]byte, error) {
	return nil, errTest
}

func (e errorDB) GetSession(ctx context.Context, sessionID string) ([]byte, error) {
	return nil, errTest
}

func (e errorDB) UpdateSession(ctx context.Context, sessionID string, payload []byte) error {
	return errTest
}

func (e errorDB) DeleteSession(ctx context.Context, sessionID string) error {
	return errTest
}

func (e errorDB) AddSessionMember(ctx context.Context, sessionID string, payload []byte) error {
	return errTest
}

func (e errorDB) ListSessionMembers(ctx context.Context, sessionID string) ([]byte, error) {
	return nil, errTest
}

func (e errorDB) IsSessionMember(ctx context.Context, sessionID, memberID string) (bool, error) {
	return false, errTest
}

func (e errorDB) CreateWorkflowCatalog(ctx context.Context, payload []byte) (string, error) {
	return "", errTest
}

func (e errorDB) ListWorkflowCatalog(ctx context.Context) ([]byte, error) {
	return nil, errTest
}

func (a approvalTrackingDB) ListContextSnapshots(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (a approvalTrackingDB) GetContextSnapshot(ctx context.Context, snapshotID string) ([]byte, error) {
	return []byte(`{"snapshot_id":"snap_1","labels":{"tenant_id":"t"},"nodes":[],"edges":[]}`), nil
}

func (a approvalTrackingDB) CreateSession(ctx context.Context, payload []byte) (string, error) {
	return "session_1", nil
}

func (a approvalTrackingDB) ListSessions(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}

func (a approvalTrackingDB) GetSession(ctx context.Context, sessionID string) ([]byte, error) {
	return []byte(`{"session_id":"session_1"}`), nil
}

func (a approvalTrackingDB) UpdateSession(ctx context.Context, sessionID string, payload []byte) error {
	return nil
}

func (a approvalTrackingDB) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (a approvalTrackingDB) AddSessionMember(ctx context.Context, sessionID string, payload []byte) error {
	return nil
}

func (a approvalTrackingDB) ListSessionMembers(ctx context.Context, sessionID string) ([]byte, error) {
	return []byte("[]"), nil
}

func (a approvalTrackingDB) IsSessionMember(ctx context.Context, sessionID, memberID string) (bool, error) {
	return true, nil
}

func (a approvalTrackingDB) CreateWorkflowCatalog(ctx context.Context, payload []byte) (string, error) {
	return "workflow_1", nil
}

func (a approvalTrackingDB) ListWorkflowCatalog(ctx context.Context) ([]byte, error) {
	return []byte("[]"), nil
}
