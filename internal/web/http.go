package web

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/db"
	"carapulse/internal/llm"
	"carapulse/internal/metrics"
	"carapulse/internal/policy"
)

// PaginationMeta carries pagination metadata in list responses.
type PaginationMeta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// parsePagination extracts limit and offset from query parameters.
// Defaults: limit=50, max limit=200, offset>=0.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

// paginatedResponse wraps data with pagination metadata.
func paginatedResponse(w http.ResponseWriter, data json.RawMessage, limit, offset, total int) {
	resp := map[string]any{
		"data": data,
		"pagination": PaginationMeta{
			Limit:  limit,
			Offset: offset,
			Total:  total,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

type Server struct {
	Mux              *http.ServeMux
	DB               DBWriter
	DBConn           *sql.DB
	Policy           *policy.Evaluator
	TemporalHealth   TemporalHealthFunc
	Goroutines       *GoroutineTracker
	Events           *EventHub
	Logs             *LogHub
	Approvals        ApprovalCreator
	Audit            AuditWriter
	Context          ContextManager
	Diagnostics      DiagnosticsCollector
	ObjectStore      ObjectStore
	Scheduler        *Scheduler
	Executor         ExecutionStarter
	Planner          llm.Planner
	EventGate        *EventGate
	WorkspaceDir     string
	AutoApproveLow   bool
	EnableEventLoop  bool
	EventLoopSources []string
	SessionRequired  bool
	FailOpenReads    bool
	RateLimiter      *RateLimiter
	eventsOnce       sync.Once
	logsOnce         sync.Once
}

var marshalJSON = json.Marshal

const maxRequestBody = 1 << 20 // 1 MB

type DBWriter interface {
	CreatePlan(ctx context.Context, planJSON []byte) (string, error)
	GetPlan(ctx context.Context, planID string) ([]byte, error)
	ListPlans(ctx context.Context, limit, offset int) ([]byte, int, error)
	CreateExecution(ctx context.Context, planID string) (string, error)
	GetExecution(ctx context.Context, execID string) ([]byte, error)
	ListExecutions(ctx context.Context, limit, offset int) ([]byte, int, error)
	CancelExecution(ctx context.Context, executionID string) error
	CreateApproval(ctx context.Context, planID string, payload []byte) (string, error)
	UpdateApprovalStatusByPlan(ctx context.Context, planID, status string) error
	ListAuditEvents(ctx context.Context, filter db.AuditFilter) ([]byte, int, error)
	ListContextServices(ctx context.Context, limit, offset int) ([]byte, int, error)
	ListContextSnapshots(ctx context.Context, limit, offset int) ([]byte, int, error)
	GetContextSnapshot(ctx context.Context, snapshotID string) ([]byte, error)
	CreateSchedule(ctx context.Context, payload []byte) (string, error)
	ListSchedules(ctx context.Context, limit, offset int) ([]byte, int, error)
	DeleteSchedule(ctx context.Context, scheduleID string) error
	CreatePlaybook(ctx context.Context, payload []byte) (string, error)
	ListPlaybooks(ctx context.Context, limit, offset int) ([]byte, int, error)
	GetPlaybook(ctx context.Context, playbookID string) ([]byte, error)
	DeletePlaybook(ctx context.Context, playbookID string) error
	CreateRunbook(ctx context.Context, payload []byte) (string, error)
	ListRunbooks(ctx context.Context, limit, offset int) ([]byte, int, error)
	GetRunbook(ctx context.Context, runbookID string) ([]byte, error)
	DeleteRunbook(ctx context.Context, runbookID string) error
	DeletePlan(ctx context.Context, planID string) error
	CreateWorkflowCatalog(ctx context.Context, payload []byte) (string, error)
	ListWorkflowCatalog(ctx context.Context, limit, offset int) ([]byte, int, error)
	CreateSession(ctx context.Context, payload []byte) (string, error)
	ListSessions(ctx context.Context, limit, offset int) ([]byte, int, error)
	GetSession(ctx context.Context, sessionID string) ([]byte, error)
	UpdateSession(ctx context.Context, sessionID string, payload []byte) error
	DeleteSession(ctx context.Context, sessionID string) error
	AddSessionMember(ctx context.Context, sessionID string, payload []byte) error
	ListSessionMembers(ctx context.Context, sessionID string) ([]byte, error)
	IsSessionMember(ctx context.Context, sessionID, memberID string) (bool, error)
}

type ExecutionStarter interface {
	StartExecution(ctx context.Context, planID, executionID string, ctxRef ContextRef, steps []PlanStep) (string, error)
}

type ExecutionWorkflowUpdater interface {
	UpdateExecutionWorkflowID(ctx context.Context, executionID, workflowID string) error
}

type ActiveExecutionChecker interface {
	HasActiveExecution(ctx context.Context, planID string) (bool, error)
}

type PlanDetailsProvider interface {
	ListPlanSteps(ctx context.Context, planID string) ([]byte, error)
	ListApprovalsByPlan(ctx context.Context, planID string) ([]byte, error)
}

type ApprovalStatusReader interface {
	GetApprovalStatus(ctx context.Context, planID string) (string, error)
}

type ApprovalTokenReader interface {
	GetApprovalStatusByToken(ctx context.Context, planID, approvalID string) (string, error)
}

type ApprovalHashWriter interface {
	SetApprovalHash(ctx context.Context, planID, hash string) error
}

type ApprovalHashReader interface {
	GetApprovalHash(ctx context.Context, planID string) (string, error)
}

type ApprovalCreator interface {
	CreateApprovalIssue(ctx context.Context, planID string) (string, error)
}

type AuditWriter interface {
	InsertAuditEvent(ctx context.Context, payload []byte) (string, error)
}

type ContextManager interface {
	RefreshContext(ctx context.Context) error
	IngestSnapshot(ctx context.Context, nodes []ctxmodel.Node, edges []ctxmodel.Edge) error
	GetServiceGraph(ctx context.Context, service string) (ctxmodel.ServiceGraph, error)
}

func NewServer(database DBWriter, evaluator *policy.Evaluator) *Server {
	s := &Server{
		Mux:    http.NewServeMux(),
		DB:     database,
		Policy: evaluator,
		Events: NewEventHub(),
		Logs:   NewLogHub(),
	}
	if c, ok := database.(interface{ Conn() *sql.DB }); ok {
		// database may be a typed-nil in tests: interface value set, underlying pointer nil.
		if conn := c.Conn(); conn != nil {
			s.DBConn = conn
		}
	}
	if auditWriter, ok := database.(AuditWriter); ok {
		s.Audit = auditWriter
	}
	if store, ok := database.(ctxmodel.Store); ok {
		s.Context = ctxmodel.NewWithStore(store)
	}
	s.registerRoutes()
	return s
}

func (s *Server) withRateLimit(h http.Handler) http.Handler {
	if s.RateLimiter == nil {
		return h
	}
	return RateLimitMiddleware(s.RateLimiter)(h)
}

func (s *Server) registerRoutes() {
	s.Mux.HandleFunc("/healthz", s.handleHealthz)
	s.Mux.HandleFunc("/readyz", s.handleReadyz)
	s.Mux.Handle("/metrics", metrics.Handler())

	// Write endpoints get rate limiting.
	s.Mux.Handle("/v1/plans", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handlePlans))))
	s.Mux.Handle("/v1/plans/", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handlePlanByID))))
	s.Mux.Handle("/v1/approvals", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleApprovals))))
	s.Mux.Handle("/v1/schedules", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleSchedules))))
	s.Mux.Handle("/v1/schedules/", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleScheduleByID))))
	s.Mux.Handle("/v1/context/refresh", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleContextRefresh))))
	s.Mux.Handle("/v1/playbooks", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handlePlaybooks))))
	s.Mux.Handle("/v1/runbooks", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleRunbooks))))
	s.Mux.Handle("/v1/memory", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleOperatorMemory))))
	s.Mux.Handle("/v1/memory/", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleOperatorMemoryByID))))
	s.Mux.Handle("/v1/sessions", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleSessions))))
	s.Mux.Handle("/v1/sessions/", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleSessionByID))))
	s.Mux.Handle("/v1/workflows", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleWorkflows))))
	s.Mux.Handle("/v1/workflows/", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleWorkflowByID))))
	s.Mux.Handle("/v1/hooks/alertmanager", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleHook))))
	s.Mux.Handle("/v1/hooks/argocd", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleHook))))
	s.Mux.Handle("/v1/hooks/git", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleHook))))
	s.Mux.Handle("/v1/hooks/k8s", s.withRateLimit(AuthMiddleware(http.HandlerFunc(s.handleHook))))

	// Read-only endpoints.
	s.Mux.Handle("/v1/audit/events", AuthMiddleware(http.HandlerFunc(s.handleAuditEvents)))
	s.Mux.Handle("/v1/context/services", AuthMiddleware(http.HandlerFunc(s.handleContextServices)))
	s.Mux.Handle("/v1/context/snapshots", AuthMiddleware(http.HandlerFunc(s.handleContextSnapshots)))
	s.Mux.Handle("/v1/context/snapshots/", AuthMiddleware(http.HandlerFunc(s.handleContextSnapshotByID)))
	s.Mux.Handle("/v1/context/graph", AuthMiddleware(http.HandlerFunc(s.handleContextGraph)))
	s.Mux.Handle("/v1/playbooks/", AuthMiddleware(http.HandlerFunc(s.handlePlaybookByID)))
	s.Mux.Handle("/v1/runbooks/", AuthMiddleware(http.HandlerFunc(s.handleRunbookByID)))
	s.Mux.Handle("/v1/executions", AuthMiddleware(http.HandlerFunc(s.handleExecutions)))
	s.Mux.Handle("/v1/executions/", AuthMiddleware(http.HandlerFunc(s.handleExecutionByID)))
	s.Mux.Handle("/v1/ws", AuthMiddleware(http.HandlerFunc(s.handleWS)))
	s.Mux.Handle("/ui/playbooks", AuthMiddleware(http.HandlerFunc(s.handleUIPlaybooks)))
	s.Mux.Handle("/ui/runbooks", AuthMiddleware(http.HandlerFunc(s.handleUIRunbooks)))
	s.Mux.Handle("/ui/plans/", AuthMiddleware(http.HandlerFunc(s.handleUIPlan)))
}

func (s *Server) eventHub() *EventHub {
	s.eventsOnce.Do(func() {
		if s.Events == nil {
			s.Events = NewEventHub()
		}
	})
	return s.Events
}

func (s *Server) logHub() *LogHub {
	s.logsOnce.Do(func() {
		if s.Logs == nil {
			s.Logs = NewLogHub()
		}
	})
	return s.Logs
}

func (s *Server) emit(event string, data any, sessionID string) {
	hub := s.eventHub()
	hub.Publish(Event{Event: event, Data: data, TS: time.Now().UTC(), SessionID: sessionID}, sessionID)
}

func (s *Server) auditEvent(ctx context.Context, action, decision string, ctxRef any, note string) {
	if s.Audit == nil {
		return
	}
	actor, _ := ActorFromContext(ctx)
	payload := map[string]any{
		"occurred_at": time.Now().UTC().Format(time.RFC3339),
		"actor":       actor,
		"action":      action,
		"decision":    decision,
		"context":     ctxRef,
	}
	if note != "" {
		payload["note"] = note
	}
	data, err := marshalJSON(payload)
	if err != nil {
		return
	}
	_, _ = s.Audit.InsertAuditEvent(ctx, data)
}

func (s *Server) createApproval(ctx context.Context, planID string, external bool) (string, error) {
	approvalID, err := s.DB.CreateApproval(ctx, planID, nil)
	if err != nil {
		return "", err
	}
	if !external || s.Approvals == nil {
		return approvalID, nil
	}
	if _, err := s.Approvals.CreateApprovalIssue(ctx, planID); err != nil {
		return approvalID, err
	}
	return approvalID, nil
}

func (s *Server) handlePlans(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
		if tenantID == "" {
			http.Error(w, "tenant_id required", http.StatusBadRequest)
			return
		}
		if err := policyCheckTenantRead(s, r, "plan.list", tenantID); err != nil {
			s.auditEvent(r.Context(), "plan.list", "deny", map[string]any{}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		limit, offset := parsePagination(r)
		payload, total, err := s.DB.ListPlans(r.Context(), limit, offset)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		filtered, err := filterJSONByTenantContext(payload, tenantID)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		paginatedResponse(w, filtered, limit, offset, total)
	case http.MethodPost:
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		requiredSession, err := s.requireSession(r)
		if err != nil {
			http.Error(w, "session denied", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var req PlanCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		risk := riskFromIntent(req.Intent)
		actionType := "read"
		if risk != "read" {
			actionType = "write"
		}
		if err := validateContextRefMinimal(req.Context); err != nil {
			s.auditEvent(r.Context(), "plan.create", "deny", req.Context, err.Error())
			http.Error(w, "invalid context", http.StatusBadRequest)
			return
		}
		dec, err := s.policyDecision(r, "plan.create", actionType, req.Context, risk, 0)
		if err != nil {
			s.auditEvent(r.Context(), "plan.create", "deny", req.Context, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		mergedConstraints := mergeConstraints(req.Constraints, dec.Constraints)
		switch dec.Decision {
		case "allow":
		case "require_approval":
			if actionType != "write" {
				s.auditEvent(r.Context(), "plan.create", "deny", req.Context, "approval required")
				http.Error(w, "policy denied", http.StatusForbidden)
				return
			}
		default:
			s.auditEvent(r.Context(), "plan.create", "deny", req.Context, "policy decision "+dec.Decision)
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		createdAt := time.Now().UTC()
		sessionID := strings.TrimSpace(req.SessionID)
		if sessionID == "" {
			if requiredSession != "" {
				sessionID = requiredSession
			} else {
				sessionID = sessionIDFromRequest(r)
			}
		}
		if requiredSession != "" && sessionID != requiredSession {
			http.Error(w, "session mismatch", http.StatusForbidden)
			return
		}
		var diagnostics []DiagnosticEvidence
		if s.Diagnostics != nil {
			if collected, err := s.Diagnostics.Collect(r.Context(), req.Context, req.Intent, req.Constraints); err == nil {
				diagnostics = collected
			}
		}
		var planText string
		if s.Planner != nil && strings.TrimSpace(req.Intent) != "" {
			planContext := map[string]any{
				"context":     req.Context,
				"constraints": req.Constraints,
				"summary":     req.Summary,
				"trigger":     req.Trigger,
				"session_id":  sessionID,
			}
			draft, err := s.Planner.Plan(req.Intent, planContext, diagnostics)
			if err != nil {
				s.auditEvent(r.Context(), "plan.create", "deny", req.Context, err.Error())
				http.Error(w, "planner error", http.StatusBadGateway)
				return
			}
			planText = draft
		}
		plan := map[string]any{
			"trigger":     req.Trigger,
			"summary":     req.Summary,
			"context":     req.Context,
			"constraints": mergedConstraints,
			"risk_level":  risk,
			"intent":      req.Intent,
			"created_at":  createdAt,
			"session_id":  sessionID,
		}
		if len(diagnostics) > 0 {
			plan["meta"] = map[string]any{"diagnostics": diagnostics}
		}
		if planText != "" {
			plan["plan_text"] = planText
			if steps := parsePlanSteps(planText); len(steps) > 0 {
				plan["steps"] = steps
			}
		}
		payload, err := marshalJSON(plan)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		planID, err := s.DB.CreatePlan(r.Context(), payload)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		plan["plan_id"] = planID
		if actionType == "write" {
			if risk == "low" && s.AutoApproveLow && dec.Decision != "require_approval" {
				approvalID, err := s.createApproval(r.Context(), planID, false)
				if err != nil {
					http.Error(w, "approval error", http.StatusBadGateway)
					return
				}
				if err := s.DB.UpdateApprovalStatusByPlan(r.Context(), planID, "approved"); err != nil {
					http.Error(w, "db error", http.StatusInternalServerError)
					return
				}
				if hashWriter, ok := s.DB.(ApprovalHashWriter); ok {
					approvedSteps, _ := parsePlanStepsPayload(plan["steps"])
					approvedIntent, _ := plan["intent"].(string)
					_ = hashWriter.SetApprovalHash(r.Context(), planID, ComputePlanHash(approvedIntent, approvedSteps))
				}
				s.auditEvent(r.Context(), "approval.auto", "allow", map[string]any{
					"plan_id":     planID,
					"approval_id": approvalID,
					"status":      "approved",
				}, "")
			} else {
				if _, err := s.createApproval(r.Context(), planID, true); err != nil {
					http.Error(w, "approval error", http.StatusBadGateway)
					return
				}
			}
		}
		s.auditEvent(r.Context(), "plan.create", "allow", map[string]any{
			"plan_id": planID,
			"context": req.Context,
			"trigger": req.Trigger,
			"intent":  req.Intent,
		}, "")
		s.emit("plan.updated", map[string]any{"plan_id": planID}, sessionID)
		resp := map[string]any{
			"plan":       plan,
			"plan_id":    planID,
			"created_at": createdAt,
		}
		writeJSON(w, http.StatusOK, resp)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePlanByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/diff") {
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		planID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/plans/"), "/diff")
		payload, err := s.DB.GetPlan(r.Context(), planID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if payload == nil {
			http.NotFound(w, r)
			return
		}
		var plan map[string]any
		if err := json.Unmarshal(payload, &plan); err != nil {
			http.Error(w, "decode error", http.StatusInternalServerError)
			return
		}
		if _, err := s.policyCheckReadPlan(r, plan, "plan.diff"); err != nil {
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		steps, _ := parsePlanStepsPayload(plan["steps"])
		diff := map[string]any{
			"plan_id": planID,
			"changes": steps,
			"targets": estimateTargets(steps),
		}
		writeJSON(w, http.StatusOK, diff)
		return
	}
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/risk") {
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		planID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/plans/"), "/risk")
		payload, err := s.DB.GetPlan(r.Context(), planID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if payload == nil {
			http.NotFound(w, r)
			return
		}
		var plan map[string]any
		if err := json.Unmarshal(payload, &plan); err != nil {
			http.Error(w, "decode error", http.StatusInternalServerError)
			return
		}
		ctxRef, err := s.policyCheckReadPlan(r, plan, "plan.risk")
		if err != nil {
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		risk := "medium"
		if val, ok := plan["risk_level"].(string); ok && strings.TrimSpace(val) != "" {
			risk = strings.TrimSpace(val)
		}
		steps, _ := parsePlanStepsPayload(plan["steps"])
		targets := estimateTargets(steps)
		out := map[string]any{
			"plan_id":      planID,
			"risk_level":   risk,
			"tier":         tierForRisk(risk),
			"blast_radius": blastRadius(ctxRef, targets),
			"targets":      targets,
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":execute") {
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		planID := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, ":execute"), "/v1/plans/")
		if strings.TrimSpace(planID) == "" {
			http.Error(w, "plan id required", http.StatusBadRequest)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		var execReq PlanExecuteRequest
		if err := json.NewDecoder(r.Body).Decode(&execReq); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		payload, err := s.DB.GetPlan(r.Context(), planID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if payload == nil {
			http.NotFound(w, r)
			return
		}
		var plan map[string]any
		if err := json.Unmarshal(payload, &plan); err != nil {
			http.Error(w, "decode error", http.StatusInternalServerError)
			return
		}
		if _, err := s.requireSession(r); err != nil {
			http.Error(w, "session denied", http.StatusForbidden)
			return
		}
		if err := enforceSessionMatch(r, plan); err != nil {
			http.Error(w, "session denied", http.StatusForbidden)
			return
		}
		risk := "medium"
		if val, ok := plan["risk_level"].(string); ok && strings.TrimSpace(val) != "" {
			risk = strings.TrimSpace(val)
		} else if intent, ok := plan["intent"].(string); ok && strings.TrimSpace(intent) != "" {
			risk = riskFromIntent(intent)
		}
		actionType := "write"
		if risk == "read" {
			actionType = "read"
		}
		ctxRef := ContextRef{}
		if ctxVal, ok := plan["context"].(map[string]any); ok {
			ctxRef = contextFromMap(ctxVal)
		}
		if err := validateContextRefStrict(ctxRef); err != nil {
			s.auditEvent(r.Context(), "plan.execute", "deny", map[string]any{"plan_id": planID}, err.Error())
			http.Error(w, "invalid context", http.StatusBadRequest)
			return
		}
		constraints := constraintsFromPlan(plan)
		steps, _ := parsePlanStepsPayload(plan["steps"])
		if len(steps) == 0 {
			if details, ok := s.DB.(PlanDetailsProvider); ok {
				if data, err := details.ListPlanSteps(r.Context(), planID); err == nil {
					parsed, err := parsePlanStepsJSON(data)
					if err == nil {
						steps = parsed
					}
				}
			}
		}
		dec, err := s.policyDecision(r, "plan.execute", actionType, ctxRef, risk, estimateTargets(steps))
		if err != nil {
			s.auditEvent(r.Context(), "plan.execute", "deny", map[string]any{"plan_id": planID}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		switch dec.Decision {
		case "allow":
		case "require_approval":
			if actionType != "write" {
				s.auditEvent(r.Context(), "plan.execute", "deny", map[string]any{"plan_id": planID}, "approval required")
				http.Error(w, "policy denied", http.StatusForbidden)
				return
			}
		case "deny":
			s.auditEvent(r.Context(), "plan.execute", "deny", map[string]any{"plan_id": planID}, "policy decision deny")
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		default:
			s.auditEvent(r.Context(), "plan.execute", "deny", map[string]any{"plan_id": planID}, "policy decision "+dec.Decision)
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		if err := enforceConstraints(ctxRef, constraints, steps, time.Now().UTC(), actionType); err != nil {
			s.auditEvent(r.Context(), "plan.execute", "deny", map[string]any{"plan_id": planID}, err.Error())
			http.Error(w, "constraints violated", http.StatusForbidden)
			return
		}
		if actionType == "write" {
			status, err := s.approvalStatus(r.Context(), planID, execReq.ApprovalToken)
			if err != nil {
				s.auditEvent(r.Context(), "plan.execute", "deny", map[string]any{"plan_id": planID}, err.Error())
				http.Error(w, "approval required", http.StatusForbidden)
				return
			}
			if status != "approved" {
				s.auditEvent(r.Context(), "plan.execute", "deny", map[string]any{"plan_id": planID}, "approval required")
				http.Error(w, "approval required", http.StatusForbidden)
				return
			}
		}
		if hashReader, ok := s.DB.(ApprovalHashReader); ok {
			approvedHash, err := hashReader.GetApprovalHash(r.Context(), planID)
			if err == nil && approvedHash != "" {
				intent, _ := plan["intent"].(string)
				currentHash := ComputePlanHash(intent, steps)
				if currentHash != approvedHash {
					s.auditEvent(r.Context(), "plan.execute", "deny", map[string]any{"plan_id": planID}, "plan modified after approval")
					http.Error(w, "plan modified after approval", http.StatusForbidden)
					return
				}
			}
		}
		if checker, ok := s.DB.(ActiveExecutionChecker); ok {
			active, err := checker.HasActiveExecution(r.Context(), planID)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			if active {
				http.Error(w, "execution already in progress", http.StatusConflict)
				return
			}
		}
		execID, err := s.DB.CreateExecution(r.Context(), planID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if s.Executor != nil {
			if workflowID, err := s.Executor.StartExecution(r.Context(), planID, execID, ctxRef, steps); err == nil {
				if updater, ok := s.DB.(ExecutionWorkflowUpdater); ok && workflowID != "" {
					_ = updater.UpdateExecutionWorkflowID(r.Context(), execID, workflowID)
				}
			}
		}
		s.auditEvent(r.Context(), "plan.execute", "allow", map[string]any{
			"plan_id":      planID,
			"execution_id": execID,
		}, "")
		planSession := sessionFromPlan(plan)
		s.emit("execution.updated", map[string]any{"execution_id": execID, "plan_id": planID}, planSession)
		writeJSON(w, http.StatusOK, map[string]any{"execution_id": execID})
		return
	}
	if r.Method == http.MethodGet {
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		planID := strings.TrimPrefix(r.URL.Path, "/v1/plans/")
		payload, err := s.DB.GetPlan(r.Context(), planID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if payload == nil {
			http.NotFound(w, r)
			return
		}
		var plan map[string]any
		if err := json.Unmarshal(payload, &plan); err != nil {
			http.Error(w, "decode error", http.StatusInternalServerError)
			return
		}
		if _, err := s.policyCheckReadPlan(r, plan, "plan.get"); err != nil {
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		if details, ok := s.DB.(PlanDetailsProvider); ok {
			steps, err := details.ListPlanSteps(r.Context(), planID)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			if len(steps) > 0 {
				plan["steps"] = json.RawMessage(steps)
			}
			approvals, err := details.ListApprovalsByPlan(r.Context(), planID)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			if len(approvals) > 0 {
				plan["approvals"] = json.RawMessage(approvals)
			}
		}
		_ = json.NewEncoder(w).Encode(plan)
		return
	}
	if r.Method == http.MethodDelete {
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		planID := strings.TrimPrefix(r.URL.Path, "/v1/plans/")
		if err := s.DB.DeletePlan(r.Context(), planID); err != nil {
			http.Error(w, "delete failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (s *Server) handleSchedules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodPost:
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		var req ScheduleCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Summary) == "" {
			http.Error(w, "summary required", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Intent) == "" {
			req.Intent = req.Summary
		}
		if strings.TrimSpace(req.Cron) == "" {
			http.Error(w, "cron required", http.StatusBadRequest)
			return
		}
		if err := validateContextRefMinimal(req.Context); err != nil {
			s.auditEvent(r.Context(), "schedule.create", "deny", req.Context, err.Error())
			http.Error(w, "invalid context", http.StatusBadRequest)
			return
		}
		risk := riskFromIntent(req.Intent)
		dec, err := s.policyDecision(r, "schedule.create", "write", req.Context, risk, 0)
		if err != nil {
			s.auditEvent(r.Context(), "schedule.create", "deny", req.Context, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		if dec.Decision != "allow" {
			s.auditEvent(r.Context(), "schedule.create", "deny", req.Context, "policy decision "+dec.Decision)
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		payload := map[string]any{
			"cron":        req.Cron,
			"context":     req.Context,
			"summary":     req.Summary,
			"intent":      req.Intent,
			"constraints": req.Constraints,
			"trigger":     "scheduled",
			"enabled":     req.Enabled,
		}
		data, err := marshalJSON(payload)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		id, err := s.DB.CreateSchedule(r.Context(), data)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		s.auditEvent(r.Context(), "schedule.create", "allow", map[string]any{
			"schedule_id": id,
			"summary":     req.Summary,
		}, "")
		writeJSON(w, http.StatusOK, map[string]any{"schedule_id": id})
	case http.MethodGet:
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		limit, offset := parsePagination(r)
		payload, total, err := s.DB.ListSchedules(r.Context(), limit, offset)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		paginatedResponse(w, payload, limit, offset, total)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleScheduleByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	scheduleID := strings.TrimPrefix(r.URL.Path, "/v1/schedules/")
	if strings.TrimSpace(scheduleID) == "" {
		http.Error(w, "schedule_id required", http.StatusBadRequest)
		return
	}
	if err := s.DB.DeleteSchedule(r.Context(), scheduleID); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleApprovals(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var req ApprovalCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.PlanID == "" {
		http.Error(w, "plan_id required", http.StatusBadRequest)
		return
	}
	status, ok := normalizeApprovalStatus(req.Status)
	if !ok {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}
	if err := s.policyCheck(r, "approval.create", "read", ContextRef{}, "read", 0); err != nil {
		s.auditEvent(r.Context(), "approval.create", "deny", map[string]any{
			"plan_id": req.PlanID,
			"status":  status,
		}, err.Error())
		http.Error(w, "policy denied", http.StatusForbidden)
		return
	}
	approvalID := ""
	if status == "pending" {
		var err error
		approvalID, err = s.createApproval(r.Context(), req.PlanID, true)
		if err != nil {
			http.Error(w, "approval error", http.StatusBadGateway)
			return
		}
	} else {
		if err := s.DB.UpdateApprovalStatusByPlan(r.Context(), req.PlanID, status); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if status == "approved" {
			if hashWriter, ok := s.DB.(ApprovalHashWriter); ok {
				if planPayload, err := s.DB.GetPlan(r.Context(), req.PlanID); err == nil && planPayload != nil {
					var planData map[string]any
					if err := json.Unmarshal(planPayload, &planData); err == nil {
						approvedSteps, _ := parsePlanStepsPayload(planData["steps"])
						approvedIntent, _ := planData["intent"].(string)
						_ = hashWriter.SetApprovalHash(r.Context(), req.PlanID, ComputePlanHash(approvedIntent, approvedSteps))
					}
				}
			}
		}
	}
	s.auditEvent(r.Context(), "approval.create", "allow", map[string]any{
		"plan_id":     req.PlanID,
		"status":      status,
		"approval_id": approvalID,
	}, "")
	writeJSON(w, http.StatusOK, map[string]any{"approval_id": approvalID, "status": status})
}

func (s *Server) handleExecutions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
	if tenantID == "" {
		http.Error(w, "tenant_id required", http.StatusBadRequest)
		return
	}
	if err := policyCheckTenantRead(s, r, "execution.list", tenantID); err != nil {
		s.auditEvent(r.Context(), "execution.list", "deny", map[string]any{}, err.Error())
		http.Error(w, "policy denied", http.StatusForbidden)
		return
	}
	limit, offset := parsePagination(r)
	payload, total, err := s.DB.ListExecutions(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	paginatedResponse(w, payload, limit, offset, total)
}

func (s *Server) handleExecutionByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/logs") {
		execID := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/logs"), "/v1/executions/")
		execID = strings.TrimSuffix(execID, "/")
		if s.DB != nil {
			if execPayload, err := s.DB.GetExecution(r.Context(), execID); err == nil && len(execPayload) > 0 {
				var exec map[string]any
				if err := json.Unmarshal(execPayload, &exec); err == nil {
					if planID, ok := exec["plan_id"].(string); ok && planID != "" {
						if planJSON, err := s.DB.GetPlan(r.Context(), planID); err == nil && len(planJSON) > 0 {
							var plan map[string]any
							if err := json.Unmarshal(planJSON, &plan); err == nil {
								if _, err := s.policyCheckReadPlan(r, plan, "execution.logs"); err != nil {
									http.Error(w, "policy denied", http.StatusForbidden)
									return
								}
							}
						}
					}
				}
			}
		}
		s.handleExecutionLogs(w, r, execID)
		return
	}
	if r.Method == http.MethodGet {
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		execID := strings.TrimPrefix(r.URL.Path, "/v1/executions/")
		payload, err := s.DB.GetExecution(r.Context(), execID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if payload == nil {
			http.NotFound(w, r)
			return
		}
		var exec map[string]any
		if err := json.Unmarshal(payload, &exec); err == nil {
			if planID, ok := exec["plan_id"].(string); ok && planID != "" {
				if planJSON, err := s.DB.GetPlan(r.Context(), planID); err == nil && len(planJSON) > 0 {
					var plan map[string]any
					if err := json.Unmarshal(planJSON, &plan); err == nil {
						if _, err := s.policyCheckReadPlan(r, plan, "execution.get"); err != nil {
							http.Error(w, "policy denied", http.StatusForbidden)
							return
						}
					}
				}
			}
		}
		w.Write(payload)
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cancel") {
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		execID := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/cancel"), "/v1/executions/")
		execID = strings.TrimSuffix(execID, "/")
		if err := s.policyCheck(r, "execution.cancel", "write", ContextRef{}, "low", 0); err != nil {
			s.auditEvent(r.Context(), "execution.cancel", "deny", map[string]any{"execution_id": execID}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		err := s.DB.CancelExecution(r.Context(), execID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		s.auditEvent(r.Context(), "execution.cancel", "allow", map[string]any{"execution_id": execID}, "")
		writeJSON(w, http.StatusOK, map[string]any{"execution_id": execID, "status": "cancelled"})
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (s *Server) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	filter, err := parseAuditFilter(r)
	if err != nil {
		http.Error(w, "invalid query", http.StatusBadRequest)
		return
	}
	payload, total, err := s.DB.ListAuditEvents(r.Context(), filter)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	paginatedResponse(w, payload, filter.Limit, filter.Offset, total)
}

func (s *Server) handleContextServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
	if tenantID == "" {
		http.Error(w, "tenant_id required", http.StatusBadRequest)
		return
	}
	if err := policyCheckTenantRead(s, r, "context.services", tenantID); err != nil {
		s.auditEvent(r.Context(), "context.services", "deny", map[string]any{}, err.Error())
		http.Error(w, "policy denied", http.StatusForbidden)
		return
	}
	limit, offset := parsePagination(r)
	payload, total, err := s.DB.ListContextServices(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	var items []map[string]any
	if err := json.Unmarshal(payload, &items); err != nil {
		http.Error(w, "decode error", http.StatusInternalServerError)
		return
	}
	items = filterItemsByLabelTenant(items, tenantID, false)
	filtered, _ := json.Marshal(items)
	paginatedResponse(w, filtered, limit, offset, total)
}

func (s *Server) handleContextSnapshots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
	if tenantID == "" {
		http.Error(w, "tenant_id required", http.StatusBadRequest)
		return
	}
	if err := policyCheckTenantRead(s, r, "context.snapshots", tenantID); err != nil {
		s.auditEvent(r.Context(), "context.snapshots", "deny", map[string]any{}, err.Error())
		http.Error(w, "policy denied", http.StatusForbidden)
		return
	}
	limit, offset := parsePagination(r)
	payload, total, err := s.DB.ListContextSnapshots(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	var items []map[string]any
	if err := json.Unmarshal(payload, &items); err != nil {
		http.Error(w, "decode error", http.StatusInternalServerError)
		return
	}
	items = filterItemsByLabelTenant(items, tenantID, false)
	filtered, _ := json.Marshal(items)
	paginatedResponse(w, filtered, limit, offset, total)
}

func (s *Server) handleContextRefresh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Context == nil {
		http.Error(w, "context unavailable", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var req ContextRefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := s.policyCheck(r, "context.refresh", "read", ContextRef{}, "read", 0); err != nil {
		s.auditEvent(r.Context(), "context.refresh", "deny", map[string]any{"service": req.Service}, err.Error())
		http.Error(w, "policy denied", http.StatusForbidden)
		return
	}
	if len(req.Nodes) > 0 || len(req.Edges) > 0 {
		if err := s.Context.IngestSnapshot(r.Context(), req.Nodes, req.Edges); err != nil {
			http.Error(w, "context error", http.StatusInternalServerError)
			return
		}
	} else {
		if err := s.Context.RefreshContext(r.Context()); err != nil {
			http.Error(w, "context error", http.StatusInternalServerError)
			return
		}
	}
	s.auditEvent(r.Context(), "context.refresh", "allow", map[string]any{
		"service":    req.Service,
		"node_count": len(req.Nodes),
		"edge_count": len(req.Edges),
	}, "")
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleContextGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Context == nil {
		http.Error(w, "context unavailable", http.StatusServiceUnavailable)
		return
	}
	service := strings.TrimSpace(r.URL.Query().Get("service"))
	if service == "" {
		http.Error(w, "service required", http.StatusBadRequest)
		return
	}
	if err := s.policyCheckRead(r, "context.graph"); err != nil {
		s.auditEvent(r.Context(), "context.graph", "deny", map[string]any{"service": service}, err.Error())
		http.Error(w, "policy denied", http.StatusForbidden)
		return
	}
	graph, err := s.Context.GetServiceGraph(r.Context(), service)
	if err != nil {
		http.Error(w, "context error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

func (s *Server) handleHook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, err := s.requireSession(r); err != nil {
		http.Error(w, "session denied", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	source := strings.TrimPrefix(r.URL.Path, "/v1/hooks/")
	eventID := ""
	if len(body) > 0 {
		sum := sha256.Sum256(body)
		eventID = hex.EncodeToString(sum[:])
	}
	s.auditEvent(r.Context(), "hook.received", "allow", map[string]any{
		"source":        source,
		"event_id":      eventID,
		"payload_bytes": len(body),
	}, "")
	if s.DB != nil {
		payload := map[string]any{}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &payload); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
		}
		if s.shouldRunEventLoop(source) {
			if s.EventGate != nil {
				allowed, fingerprint, err := s.EventGate.Accept(r.Context(), source, payload)
				if err != nil {
					s.auditEvent(r.Context(), "event.gate", "deny", map[string]any{"source": source, "fingerprint": fingerprint}, err.Error())
					http.Error(w, "event gate error", http.StatusBadRequest)
					return
				}
				if !allowed {
					s.auditEvent(r.Context(), "event.gate", "deny", map[string]any{"source": source, "fingerprint": fingerprint}, "suppressed")
					w.WriteHeader(http.StatusAccepted)
					_ = json.NewEncoder(w).Encode(map[string]any{"received": true, "gated": true, "event_id": eventID})
					return
				}
			}
			res, err := s.runAlertEventLoop(r.Context(), source, payload)
			if err != nil {
				s.auditEvent(r.Context(), "event.loop", "deny", map[string]any{"source": source}, err.Error())
				http.Error(w, "event loop error", http.StatusBadRequest)
				return
			}
			s.auditEvent(r.Context(), "event.loop", "allow", map[string]any{
				"source":       source,
				"plan_id":      res.PlanID,
				"execution_id": res.ExecutionID,
			}, "")
			_ = json.NewEncoder(w).Encode(map[string]any{"received": true, "plan_id": res.PlanID, "execution_id": res.ExecutionID})
			return
		}
		ctxRef := contextFromHook(payload)
		summary := hookSummary(source, payload)
		intent := summary
		risk := riskFromIntent(intent)
		actionType := "read"
		if risk != "read" {
			actionType = "write"
		}
		dec, err := s.policyDecision(r, "plan.create", actionType, ctxRef, risk, 0)
		if err != nil {
			s.auditEvent(r.Context(), "plan.create", "deny", ctxRef, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		mergedConstraints := mergeConstraints(nil, dec.Constraints)
		switch dec.Decision {
		case "allow":
		case "require_approval":
			if actionType != "write" {
				s.auditEvent(r.Context(), "plan.create", "deny", ctxRef, "approval required")
				http.Error(w, "policy denied", http.StatusForbidden)
				return
			}
		default:
			s.auditEvent(r.Context(), "plan.create", "deny", ctxRef, "policy decision "+dec.Decision)
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		createdAt := time.Now().UTC()
		var planText string
		if s.Planner != nil && strings.TrimSpace(intent) != "" {
			planContext := map[string]any{
				"context": ctxRef,
				"summary": summary,
				"source":  source,
				"payload": payload,
				"trigger": "webhook",
			}
			draft, err := s.Planner.Plan(intent, planContext, nil)
			if err != nil {
				s.auditEvent(r.Context(), "plan.create", "deny", ctxRef, err.Error())
				http.Error(w, "planner error", http.StatusBadGateway)
				return
			}
			planText = draft
		}
		plan := map[string]any{
			"trigger":     "webhook",
			"summary":     summary,
			"context":     ctxRef,
			"risk_level":  risk,
			"intent":      intent,
			"constraints": mergedConstraints,
			"created_at":  createdAt,
		}
		if planText != "" {
			plan["plan_text"] = planText
			if steps := parsePlanSteps(planText); len(steps) > 0 {
				plan["steps"] = steps
			}
		}
		data, err := marshalJSON(plan)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		planID, err := s.DB.CreatePlan(r.Context(), data)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		// Webhook-triggered plans always require human approval regardless
		// of risk level. The risk classification is based on intent keywords
		// which can be gamed, and LLM-generated plans are not trusted.
		if actionType == "write" {
			if _, err := s.createApproval(r.Context(), planID, true); err != nil {
				http.Error(w, "approval error", http.StatusBadGateway)
				return
			}
		}
		s.auditEvent(r.Context(), "plan.create", "allow", map[string]any{
			"plan_id":  planID,
			"context":  ctxRef,
			"trigger":  "webhook",
			"summary":  summary,
			"source":   source,
			"event_id": eventID,
		}, "")
		s.emit("plan.updated", map[string]any{"plan_id": planID}, sessionIDFromRequest(r))
	}
	_ = json.NewEncoder(w).Encode(HookAck{Received: true, EventID: eventID})
}

func (s *Server) approvalStatus(ctx context.Context, planID, approvalToken string) (string, error) {
	if strings.TrimSpace(planID) == "" {
		return "", errors.New("plan id required")
	}
	if s.DB == nil {
		return "", errors.New("db unavailable")
	}
	if token := strings.TrimSpace(approvalToken); token != "" {
		if reader, ok := s.DB.(ApprovalTokenReader); ok {
			return reader.GetApprovalStatusByToken(ctx, planID, token)
		}
		return "", errors.New("approval token unsupported")
	}
	if reader, ok := s.DB.(ApprovalStatusReader); ok {
		return reader.GetApprovalStatus(ctx, planID)
	}
	return "", errors.New("approval status unavailable")
}

// ComputePlanHash computes a SHA-256 hash over the plan's intent and steps.
// The hash is stored at approval time and verified before execution to detect
// plan tampering between approval and execution (SEC-01).
func ComputePlanHash(intent string, steps []PlanStep) string {
	normalized := make([]PlanStep, len(steps))
	copy(normalized, steps)
	for i := range normalized {
		if strings.TrimSpace(normalized[i].Stage) == "" {
			normalized[i].Stage = "act"
		}
	}
	canonical := struct {
		Intent string     `json:"intent"`
		Steps  []PlanStep `json:"steps"`
	}{Intent: intent, Steps: normalized}
	data, err := json.Marshal(canonical)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hookSummary(source string, payload map[string]any) string {
	switch source {
	case "alertmanager":
		if name := extractAlertName(payload); name != "" {
			return "Alertmanager: " + name
		}
		return "Alertmanager webhook"
	case "argocd":
		return "Argo CD webhook"
	case "git":
		return "Git webhook"
	default:
		if source == "" {
			return "Webhook"
		}
		return "Webhook: " + source
	}
}

func (s *Server) shouldRunEventLoop(source string) bool {
	if !s.EnableEventLoop {
		return false
	}
	if len(s.EventLoopSources) == 0 {
		switch source {
		case "alertmanager", "argocd", "git", "k8s":
			return true
		default:
			return false
		}
	}
	for _, allowed := range s.EventLoopSources {
		if strings.EqualFold(strings.TrimSpace(allowed), source) {
			return true
		}
	}
	return false
}

func contextFromHook(payload map[string]any) ContextRef {
	if payload == nil {
		return ContextRef{}
	}
	if ctxRaw, ok := payload["context"]; ok {
		if ctx, ok := ctxRaw.(map[string]any); ok {
			return contextFromMap(ctx)
		}
	}
	if labels := extractAlertLabels(payload); len(labels) > 0 {
		return contextFromMap(labels)
	}
	return ContextRef{}
}

func contextFromMap(data map[string]any) ContextRef {
	return ContextRef{
		TenantID:      stringFromMap(data, "tenant_id", "tenant", "org"),
		Environment:   stringFromMap(data, "environment", "env"),
		ClusterID:     stringFromMap(data, "cluster_id", "cluster"),
		Namespace:     stringFromMap(data, "namespace"),
		AWSAccountID:  stringFromMap(data, "aws_account_id", "account_id"),
		Region:        stringFromMap(data, "region"),
		ArgoCDProject: stringFromMap(data, "argocd_project", "project"),
		GrafanaOrgID:  stringFromMap(data, "grafana_org_id", "grafana_org"),
	}
}

func stringFromMap(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := data[key]; ok {
			if v, ok := raw.(string); ok {
				return v
			}
		}
	}
	return ""
}

func extractAlertName(payload map[string]any) string {
	if labels := extractAlertLabels(payload); len(labels) > 0 {
		if name, ok := labels["alertname"].(string); ok {
			return name
		}
	}
	if common, ok := payload["commonLabels"].(map[string]any); ok {
		if name, ok := common["alertname"].(string); ok {
			return name
		}
	}
	return ""
}

func extractAlertSeverity(payload map[string]any) string {
	if labels := extractAlertLabels(payload); len(labels) > 0 {
		if name, ok := labels["severity"].(string); ok {
			return name
		}
	}
	if common, ok := payload["commonLabels"].(map[string]any); ok {
		if name, ok := common["severity"].(string); ok {
			return name
		}
	}
	return ""
}

func extractAlertLabels(payload map[string]any) map[string]any {
	alerts, ok := payload["alerts"].([]any)
	if !ok || len(alerts) == 0 {
		return nil
	}
	first, ok := alerts[0].(map[string]any)
	if !ok {
		return nil
	}
	labels, _ := first["labels"].(map[string]any)
	return labels
}

func parseAuditFilter(r *http.Request) (db.AuditFilter, error) {
	query := r.URL.Query()
	var filter db.AuditFilter
	if from := query.Get("from"); from != "" {
		parsed, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return filter, err
		}
		filter.From = parsed
	}
	if to := query.Get("to"); to != "" {
		parsed, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return filter, err
		}
		filter.To = parsed
	}
	filter.ActorID = query.Get("actor_id")
	filter.Action = query.Get("action")
	filter.Decision = query.Get("decision")
	filter.Limit, filter.Offset = parsePagination(r)
	return filter, nil
}
