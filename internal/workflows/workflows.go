package workflows

import (
	"context"
)

func GitOpsDeployWorkflow(ctx context.Context, in DeployInput, rt *Runtime, database DBReader) error {
	if err := RequireApproval(ctx, database, in.PlanID); err != nil {
		return err
	}
	if err := ArgoSyncDryRunActivity(ctx, in.ArgoCDApp, rt); err != nil {
		return err
	}
	_ = ArgoSyncPreviewActivity(ctx, in.ArgoCDApp, rt)
	if err := ArgoSyncActivity(ctx, in.ArgoCDApp, rt); err != nil {
		if in.Revision != "" {
			_ = ArgoRollbackActivity(ctx, in.ArgoCDApp, in.Revision, rt)
		}
		return err
	}
	if err := ArgoWaitActivity(ctx, in.ArgoCDApp, rt); err != nil {
		if in.Revision != "" {
			_ = ArgoRollbackActivity(ctx, in.ArgoCDApp, in.Revision, rt)
		}
		return err
	}
	return nil
}

func HelmReleaseWorkflow(ctx context.Context, in HelmInput, rt *Runtime, database DBReader) error {
	if err := RequireApproval(ctx, database, in.PlanID); err != nil {
		return err
	}
	if in.Strategy == "rollback" {
		return HelmRollbackActivity(ctx, in.Release, rt)
	}
	if err := HelmUpgradeActivity(ctx, in.Release, rt); err != nil {
		_ = HelmRollbackActivity(ctx, in.Release, rt)
		return err
	}
	_, _ = HelmStatusActivity(ctx, in.Release, rt)
	return nil
}

func ScaleServiceWorkflow(ctx context.Context, in ScaleInput, rt *Runtime, database DBReader) error {
	if err := RequireApproval(ctx, database, in.PlanID); err != nil {
		return err
	}
	if err := KubectlScaleActivity(ctx, in.Service, in.Replicas, rt); err != nil {
		return err
	}
	return KubectlRolloutStatusActivity(ctx, in.Service, rt)
}

func IncidentRemediationWorkflow(ctx context.Context, in IncidentInput, rt *Runtime, database DBReader) error {
	if err := RequireApproval(ctx, database, in.PlanID); err != nil {
		return err
	}
	_, _ = PromRulesActivity(ctx, rt)
	if in.Service != "" {
		_, _ = QueryPrometheusActivity(ctx, "up", rt)
	}
	return nil
}

func SecretRotationWorkflow(ctx context.Context, in SecretRotationInput, rt *Runtime, database DBReader) error {
	if err := RequireApproval(ctx, database, in.PlanID); err != nil {
		return err
	}
	if in.SecretPath == "" {
		return nil
	}
	_ = VaultRenewActivity(ctx, in.SecretPath, rt)
	_ = VaultRevokeActivity(ctx, in.SecretPath, rt)
	return nil
}
