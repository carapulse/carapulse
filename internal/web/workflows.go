package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func findWorkflowByName(payload []byte, name string) (map[string]any, bool) {
	var items []map[string]any
	if err := json.Unmarshal(payload, &items); err != nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false
	}
	var selected map[string]any
	maxVersion := 0
	for _, item := range items {
		if n, ok := item["name"].(string); ok && strings.EqualFold(n, name) {
			version := 0
			switch v := item["version"].(type) {
			case int:
				version = v
			case float64:
				version = int(v)
			}
			if version >= maxVersion {
				maxVersion = version
				selected = item
			}
		}
	}
	if selected == nil {
		return nil, false
	}
	return selected, true
}

func (s *Server) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
	if tenantID == "" {
		http.Error(w, "tenant_id required", http.StatusBadRequest)
		return
	}
	if err := policyCheckTenantRead(s, r, "workflow.list", tenantID); err != nil {
		s.auditEvent(r.Context(), "workflow.list", "deny", map[string]any{}, err.Error())
		http.Error(w, "policy denied", http.StatusForbidden)
		return
	}
	if s.DB != nil {
		if payload, err := s.DB.ListWorkflowCatalog(r.Context()); err == nil && len(payload) > 0 {
			filtered, err := filterJSONByTenant(payload, tenantID, true)
			if err != nil {
				http.Error(w, "encode error", http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"workflows": json.RawMessage(filtered)})
			return
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"workflows": workflowCatalog()})
}

func (s *Server) handleWorkflowByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/v1/workflows/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "workflow required", http.StatusBadRequest)
		return
	}
	name := parts[0]
	if len(parts) == 1 && r.Method == http.MethodGet {
		tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
		if tenantID == "" {
			http.Error(w, "tenant_id required", http.StatusBadRequest)
			return
		}
		if err := policyCheckTenantRead(s, r, "workflow.get", tenantID); err != nil {
			s.auditEvent(r.Context(), "workflow.get", "deny", map[string]any{"workflow": name}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		if s.DB != nil {
			if payload, err := s.DB.ListWorkflowCatalog(r.Context()); err == nil && len(payload) > 0 {
				filtered, err := filterJSONByTenant(payload, tenantID, true)
				if err != nil {
					http.Error(w, "encode error", http.StatusInternalServerError)
					return
				}
				if found, ok := findWorkflowByName(filtered, name); ok {
					_ = json.NewEncoder(w).Encode(found)
					return
				}
			}
		}
		template, ok := findWorkflowTemplate(name)
		if !ok {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(template)
		return
	}
	if len(parts) == 2 && parts[1] == "version" && r.Method == http.MethodPost {
		if s.DB == nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		bodyTenant, _ := req["tenant_id"].(string)
		tenantID, err := resolveTenant(bodyTenant, r.Header.Get("X-Tenant-Id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.policyCheck(r, "workflow.version.create", "write", ContextRef{TenantID: tenantID}, "low", 0); err != nil {
			s.auditEvent(r.Context(), "workflow.version.create", "deny", map[string]any{"workflow": name}, err.Error())
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
		req["name"] = name
		req["tenant_id"] = tenantID
		data, err := marshalJSON(req)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		id, err := s.DB.CreateWorkflowCatalog(r.Context(), data)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		s.auditEvent(r.Context(), "workflow.version.create", "allow", map[string]any{"workflow": name, "workflow_id": id}, "")
		_ = json.NewEncoder(w).Encode(map[string]any{"workflow_id": id})
		return
	}
	if len(parts) != 2 || parts[1] != "start" || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	template, ok := findWorkflowTemplate(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var req WorkflowStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Input == nil {
		req.Input = map[string]any{}
	}
	if err := validateContextRefStrict(req.Context); err != nil {
		s.auditEvent(r.Context(), "workflow.start", "deny", req.Context, err.Error())
		http.Error(w, "invalid context", http.StatusBadRequest)
		return
	}
	summary, steps, err := buildWorkflowPlan(name, req.Input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	intent := "Workflow " + name
	risk := template.Risk
	if strings.TrimSpace(risk) == "" {
		risk = "low"
	}
	actionType := "read"
	if risk != "read" {
		actionType = "write"
	}
	dec, err := s.policyDecision(r, "plan.create", actionType, req.Context, risk, 0)
	if err != nil {
		s.auditEvent(r.Context(), "workflow.start", "deny", req.Context, err.Error())
		http.Error(w, "policy denied", http.StatusForbidden)
		return
	}
	switch dec.Decision {
	case "allow":
	case "require_approval":
		if actionType != "write" {
			s.auditEvent(r.Context(), "workflow.start", "deny", req.Context, "approval required")
			http.Error(w, "policy denied", http.StatusForbidden)
			return
		}
	default:
		s.auditEvent(r.Context(), "workflow.start", "deny", req.Context, "policy decision "+dec.Decision)
		http.Error(w, "policy denied", http.StatusForbidden)
		return
	}
	plan := map[string]any{
		"trigger":     "workflow",
		"summary":     summary,
		"context":     req.Context,
		"risk_level":  risk,
		"intent":      intent,
		"constraints": mergeConstraints(req.Constraints, dec.Constraints),
		"created_at":  time.Now().UTC(),
		"steps":       steps,
		"meta": map[string]any{
			"workflow": name,
			"input":    req.Input,
		},
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
	approvalRequired := actionType == "write"
	execID := ""
	if approvalRequired {
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
			s.auditEvent(r.Context(), "approval.auto", "allow", map[string]any{
				"plan_id":     planID,
				"approval_id": approvalID,
				"status":      "approved",
			}, "")
		} else if _, err := s.createApproval(r.Context(), planID, true); err != nil {
			http.Error(w, "approval error", http.StatusBadGateway)
			return
		}
	}
	if s.Executor != nil && (!approvalRequired || risk == "low" && s.AutoApproveLow) {
		execID, err = s.DB.CreateExecution(r.Context(), planID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if workflowID, err := s.Executor.StartExecution(r.Context(), planID, execID, req.Context, steps); err == nil {
			if updater, ok := s.DB.(ExecutionWorkflowUpdater); ok && workflowID != "" {
				_ = updater.UpdateExecutionWorkflowID(r.Context(), execID, workflowID)
			}
		}
	}
	s.auditEvent(r.Context(), "workflow.start", "allow", map[string]any{"plan_id": planID, "workflow": name}, "")
	_ = json.NewEncoder(w).Encode(WorkflowStartResponse{PlanID: planID, ExecutionID: execID, Status: "ok"})
}
