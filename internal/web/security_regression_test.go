package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"carapulse/internal/auth"
	"carapulse/internal/policy"
	"carapulse/internal/tools"
)

// SEC-01: Plan Tampering Between Approval and Execution
// Tests that if a plan's steps are modified after approval, execution is rejected.

// hashDB implements ApprovalHashReader and ApprovalHashWriter so the
// plan-integrity check in handlePlanByID can store and verify hashes.
type hashDB struct {
	fakeDB
	hash string
}

func (h *hashDB) GetApprovalHash(ctx context.Context, planID string) (string, error) {
	return h.hash, nil
}

func (h *hashDB) SetApprovalHash(ctx context.Context, planID, hash string) error {
	h.hash = hash
	return nil
}

func (h *hashDB) HasActiveExecution(ctx context.Context, planID string) (bool, error) {
	return false, nil
}

func TestSEC01_PlanTamperDetection(t *testing.T) {
	enableDevMode(t)

	// Build a plan with known steps.
	steps := []PlanStep{
		{StepID: "s1", Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "deploy/app", "replicas": 3}},
	}
	plan := map[string]any{
		"plan_id":    "plan_1",
		"intent":     "scale service",
		"risk_level": "low",
		"context":    validContext(),
		"steps":      steps,
	}
	planJSON, _ := json.Marshal(plan)

	// Compute the original hash that would be stored at approval time.
	originalHash := ComputePlanHash("scale service", steps)

	db := &hashDB{
		fakeDB: fakeDB{planID: "plan_1", lastPlan: planJSON},
		hash:   originalHash,
	}

	// Now tamper with the plan: modify the steps in the DB payload.
	tamperedSteps := []PlanStep{
		{StepID: "s1", Tool: "kubectl", Action: "delete", Input: map[string]any{"resource": "namespace/production"}},
	}
	tamperedPlan := map[string]any{
		"plan_id":    "plan_1",
		"intent":     "scale service",
		"risk_level": "low",
		"context":    validContext(),
		"steps":      tamperedSteps,
	}
	tamperedJSON, _ := json.Marshal(tamperedPlan)
	db.lastPlan = tamperedJSON

	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("SEC-01: tampered plan should be rejected, got status %d body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "plan modified after approval") {
		t.Fatalf("SEC-01: expected 'plan modified after approval' message, got: %s", w.Body.String())
	}
}

func TestSEC01_PlanHashMatchAllowsExecution(t *testing.T) {
	enableDevMode(t)

	// Build the plan JSON, then re-parse the steps from JSON the same way the
	// handler does, to ensure hash stability through JSON round-tripping.
	steps := []PlanStep{
		{StepID: "s1", Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "deploy/app", "replicas": float64(3)}},
	}
	plan := map[string]any{
		"plan_id":    "plan_1",
		"intent":     "scale service",
		"risk_level": "low",
		"context":    validContext(),
		"steps":      steps,
	}
	planJSON, _ := json.Marshal(plan)

	// Simulate the round-trip: unmarshal, re-parse steps, compute hash.
	var roundTripped map[string]any
	_ = json.Unmarshal(planJSON, &roundTripped)
	parsedSteps, _ := parsePlanStepsPayload(roundTripped["steps"])
	originalHash := ComputePlanHash("scale service", parsedSteps)

	db := &hashDB{
		fakeDB: fakeDB{planID: "plan_1", lastPlan: planJSON},
		hash:   originalHash,
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/plans/plan_1:execute", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{Mux: http.NewServeMux(), DB: db, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	AuthMiddleware(http.HandlerFunc(server.handlePlanByID)).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("SEC-01: unmodified plan should be allowed, got status %d body: %s", w.Code, w.Body.String())
	}
}

func TestSEC01_ComputePlanHashDeterministic(t *testing.T) {
	steps := []PlanStep{
		{StepID: "s1", Tool: "kubectl", Action: "scale"},
		{StepID: "s2", Tool: "helm", Action: "upgrade"},
	}
	h1 := ComputePlanHash("intent", steps)
	h2 := ComputePlanHash("intent", steps)
	if h1 != h2 {
		t.Fatalf("SEC-01: same input must produce same hash")
	}
	if h1 == "" {
		t.Fatalf("SEC-01: hash must not be empty")
	}
}

func TestSEC01_ComputePlanHashDiffersOnChange(t *testing.T) {
	steps1 := []PlanStep{{StepID: "s1", Tool: "kubectl", Action: "scale"}}
	steps2 := []PlanStep{{StepID: "s1", Tool: "kubectl", Action: "delete"}}
	h1 := ComputePlanHash("intent", steps1)
	h2 := ComputePlanHash("intent", steps2)
	if h1 == h2 {
		t.Fatalf("SEC-01: different steps must produce different hashes")
	}

	// Different intent, same steps.
	h3 := ComputePlanHash("other intent", steps1)
	if h1 == h3 {
		t.Fatalf("SEC-01: different intent must produce different hashes")
	}
}

// SEC-02: JWT Bypass -- JWKS URL empty + DevMode off must reject.

func TestSEC02_EmptyJWKSNoDevModeRejects(t *testing.T) {
	SetAuthConfig(AuthConfig{})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	token := tokenForClaims(t, map[string]any{
		"sub":   "attacker",
		"email": "attacker@evil.com",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)

	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("SEC-02: empty JWKS URL without DevMode must reject, got %d", rw.Code)
	}
}

func TestSEC02_ForgedJWTRejected(t *testing.T) {
	SetAuthConfig(AuthConfig{JWKSURL: "http://fake-jwks"})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	// Use a forged token (no real signature).
	token := tokenForClaims(t, map[string]any{
		"sub":    "admin",
		"email":  "admin@company.com",
		"groups": []string{"sre", "admin"},
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()

	oldFetch := auth.FetchJWKS
	auth.FetchJWKS = func(url string) (auth.JWKS, error) {
		return auth.JWKS{Keys: []auth.JWK{}}, nil
	}
	t.Cleanup(func() { auth.FetchJWKS = oldFetch })

	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)

	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("SEC-02: forged JWT must be rejected, got %d", rw.Code)
	}
}

func TestSEC02_DevModeSkipsSignatureVerification(t *testing.T) {
	SetAuthConfig(AuthConfig{DevMode: true})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	token := tokenForClaims(t, map[string]any{
		"sub": "dev-user",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("SEC-02: DevMode should accept unsigned tokens, got %d", rw.Code)
	}
}

func TestSEC02_ValidSignedTokenAccepted(t *testing.T) {
	key := newRSAKey(t)
	token := signToken(t, key, "kid", map[string]any{"sub": "user1"})

	SetAuthConfig(AuthConfig{JWKSURL: "http://jwks"})
	t.Cleanup(func() { SetAuthConfig(AuthConfig{DevMode: true}) })

	oldFetch := auth.FetchJWKS
	auth.FetchJWKS = func(url string) (auth.JWKS, error) {
		return auth.JWKS{Keys: []auth.JWK{jwkForKey(key.PublicKey, "kid")}}, nil
	}
	t.Cleanup(func() { auth.FetchJWKS = oldFetch })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("SEC-02: valid signed token should be accepted, got %d", rw.Code)
	}
}

// SEC-10 / Auto-exec Chain: LLM-generated plans always require human approval.

func TestSEC10_LLMPlanNeverAutoExecuted(t *testing.T) {
	db := &approvalDB{}
	exec := &execStub{}
	s := &Server{
		DB:              db,
		Policy:          &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) { return policy.PolicyDecision{Decision: "allow"}, nil })},
		Planner:         plannerStub{text: "1. deploy\n2. verify"},
		Executor:        exec,
		AutoApproveLow:  true,
		EnableEventLoop: true,
	}
	payload := validHookPayload()
	payload["alerts"] = []any{map[string]any{"labels": map[string]any{"alertname": "deploy service"}}}

	res, err := s.runAlertEventLoop(context.Background(), "alertmanager", payload)
	if err != nil {
		t.Fatalf("SEC-10: err: %v", err)
	}

	// Plan must be created but never auto-executed.
	if res.PlanID == "" {
		t.Fatalf("SEC-10: plan must be created")
	}
	if res.ExecutionID != "" {
		t.Fatalf("SEC-10: LLM plan must never be auto-executed, got execution_id=%s", res.ExecutionID)
	}
	if exec.calls != 0 {
		t.Fatalf("SEC-10: executor must not be called for LLM plans, got %d calls", exec.calls)
	}
	if db.created == 0 {
		t.Fatalf("SEC-10: approval must always be created for LLM plans")
	}
}

func TestSEC10_RiskEscalationFromSteps(t *testing.T) {
	// A plan with "deploy" intent (low risk) but containing high-risk steps
	// like kubectl delete should be escalated.
	db := &approvalDB{}
	s := &Server{
		DB:     db,
		Policy: &policy.Evaluator{Checker: policy.CheckerFunc(func(input policy.PolicyInput) (policy.PolicyDecision, error) { return policy.PolicyDecision{Decision: "allow"}, nil })},
		Planner: plannerStub{text: `[{"tool":"kubectl","action":"delete","input":{"resource":"namespace/prod"}}]`},
	}
	payload := validHookPayload()
	payload["alerts"] = []any{map[string]any{"labels": map[string]any{"alertname": "deploy fix"}}}

	res, err := s.runAlertEventLoop(context.Background(), "alertmanager", payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.PlanID == "" {
		t.Fatalf("plan must be created")
	}
	// Parse the stored plan to check risk was escalated.
	var storedPlan map[string]any
	if err := json.Unmarshal(db.lastPlan, &storedPlan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	risk, _ := storedPlan["risk_level"].(string)
	if risk != "high" {
		t.Fatalf("SEC-10: risk should be escalated to 'high' based on kubectl delete step, got %q", risk)
	}
}

// SEC-03: NewSandboxWithConfig must default Enforce=true.
// This is the constructor used by production services (orchestrator, tool-router).

func TestSEC03_NewSandboxWithConfigDefaultsEnforceTrue(t *testing.T) {
	sb := tools.NewSandboxWithConfig(true, "docker", "img", nil, nil)
	if !sb.Enforce {
		t.Fatalf("SEC-03: NewSandboxWithConfig must default Enforce=true")
	}
}

func TestSEC03_NewSandboxWithConfigDisabledRejectsExecution(t *testing.T) {
	// When sandbox is not enabled but Enforce is true, execution must
	// be rejected with "sandbox required".
	sb := tools.NewSandboxWithConfig(false, "", "", nil, nil)
	_, err := sb.Run(context.Background(), []string{"echo", "test"})
	if err == nil {
		t.Fatalf("SEC-03: sandbox with Enforce=true and Enabled=false must reject execution")
	}
	if err.Error() != "sandbox required" {
		t.Fatalf("SEC-03: expected 'sandbox required', got: %v", err)
	}
}

// SEC-10: handleHook non-event-loop path must never auto-approve webhook plans.

func TestSEC10_HandleHookNeverAutoApprovesWebhookPlans(t *testing.T) {
	enableDevMode(t)
	db := &fakeDB{}
	approvals := &fakeApprovalCreator{}
	// Craft a webhook payload that would be classified as "low" risk
	// via intent keywords. Even with AutoApproveLow=true, webhook plans
	// must NOT be auto-approved.
	body := []byte(`{"event":"deploy"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/deploy", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	server := &Server{
		Mux:            http.NewServeMux(),
		DB:             db,
		Approvals:      approvals,
		AutoApproveLow: true,
		Policy:         &policy.Evaluator{Checker: allowChecker{}},
	}
	AuthMiddleware(http.HandlerFunc(server.handleHook)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SEC-10: status %d, body: %s", w.Code, w.Body.String())
	}
	if db.updateStatus == "approved" {
		t.Fatalf("SEC-10: webhook plan must never be auto-approved even when AutoApproveLow=true")
	}
	if !approvals.called {
		t.Fatalf("SEC-10: external approval must be created for webhook-triggered plans")
	}
}
