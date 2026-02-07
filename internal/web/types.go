package web

import ctxmodel "carapulse/internal/context"

type ContextRef struct {
	TenantID      string `json:"tenant_id"`
	Environment   string `json:"environment"`
	ClusterID     string `json:"cluster_id"`
	Namespace     string `json:"namespace"`
	AWSAccountID  string `json:"aws_account_id"`
	Region        string `json:"region"`
	ArgoCDProject string `json:"argocd_project"`
	GrafanaOrgID  string `json:"grafana_org_id"`
}

type PlanCreateRequest struct {
	Summary     string     `json:"summary"`
	Trigger     string     `json:"trigger"`
	Context     ContextRef `json:"context"`
	Intent      string     `json:"intent"`
	Constraints any        `json:"constraints"`
	SessionID   string     `json:"session_id"`
}

type PlanExecuteRequest struct {
	PlanID        string `json:"plan_id"`
	ApprovalToken string `json:"approval_token"`
}

type ScheduleCreateRequest struct {
	Summary     string     `json:"summary"`
	Cron        string     `json:"cron"`
	Context     ContextRef `json:"context"`
	Intent      string     `json:"intent"`
	Constraints any        `json:"constraints"`
	Enabled     bool       `json:"enabled"`
}

type ApprovalCreateRequest struct {
	PlanID       string `json:"plan_id"`
	ApproverNote string `json:"approver_note"`
	Status       string `json:"status"`
}

type PlaybookCreateRequest struct {
	TenantID string   `json:"tenant_id"`
	Name     string   `json:"name"`
	Version  int      `json:"version"`
	Tags     []string `json:"tags"`
	Spec     any      `json:"spec"`
}

type AuditQueryRequest struct {
	From     string `json:"from"`
	To       string `json:"to"`
	ActorID  string `json:"actor_id"`
	Action   string `json:"action"`
	Decision string `json:"decision"`
}

type HookAck struct {
	Received bool   `json:"received"`
	EventID  string `json:"event_id"`
}

type ContextRefreshRequest struct {
	Service string          `json:"service"`
	Nodes   []ctxmodel.Node `json:"nodes"`
	Edges   []ctxmodel.Edge `json:"edges"`
}
