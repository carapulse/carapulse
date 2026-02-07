package tools

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
