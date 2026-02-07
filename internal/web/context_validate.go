package web

import (
	"errors"
	"strings"
)

func validateContextRefStrict(ctx ContextRef) error {
	if strings.TrimSpace(ctx.TenantID) == "" {
		return errors.New("tenant_id required")
	}
	if strings.TrimSpace(ctx.Environment) == "" {
		return errors.New("environment required")
	}
	if strings.TrimSpace(ctx.ClusterID) == "" {
		return errors.New("cluster_id required")
	}
	if strings.TrimSpace(ctx.Namespace) == "" {
		return errors.New("namespace required")
	}
	if strings.TrimSpace(ctx.AWSAccountID) == "" {
		return errors.New("aws_account_id required")
	}
	if strings.TrimSpace(ctx.Region) == "" {
		return errors.New("region required")
	}
	if strings.TrimSpace(ctx.ArgoCDProject) == "" {
		return errors.New("argocd_project required")
	}
	if strings.TrimSpace(ctx.GrafanaOrgID) == "" {
		return errors.New("grafana_org_id required")
	}
	return nil
}
