package workflows

import "carapulse/internal/tools"

func contextFromMap(values map[string]any) tools.ContextRef {
	if values == nil {
		return tools.ContextRef{}
	}
	get := func(key string) string {
		val, _ := values[key].(string)
		return val
	}
	return tools.ContextRef{
		TenantID:      get("tenant_id"),
		Environment:   get("environment"),
		ClusterID:     get("cluster_id"),
		Namespace:     get("namespace"),
		AWSAccountID:  get("aws_account_id"),
		Region:        get("region"),
		ArgoCDProject: get("argocd_project"),
		GrafanaOrgID:  get("grafana_org_id"),
	}
}
