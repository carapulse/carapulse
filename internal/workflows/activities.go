package workflows

import (
	"context"
	"encoding/json"
	"errors"
)

func QueryPrometheusActivity(ctx context.Context, query string, rt *Runtime) ([]byte, error) {
	input := map[string]any{"query": query}
	return runTool(ctx, rt, "prometheus", "query", input)
}

func QueryTempoActivity(ctx context.Context, query string, rt *Runtime) ([]byte, error) {
	input := map[string]any{"query": query}
	return runTool(ctx, rt, "tempo", "query", input)
}

func ArgoSyncActivity(ctx context.Context, app string, rt *Runtime) error {
	_, err := rt.RunCLI(ctx, "argocd", "sync", map[string]any{"app": app})
	return err
}

func ArgoSyncDryRunActivity(ctx context.Context, app string, rt *Runtime) error {
	_, err := rt.RunCLI(ctx, "argocd", "sync-dry-run", map[string]any{"app": app})
	return err
}

func ArgoSyncPreviewActivity(ctx context.Context, app string, rt *Runtime) error {
	_, err := rt.RunCLI(ctx, "argocd", "sync-preview", map[string]any{"app": app})
	return err
}

func ArgoWaitActivity(ctx context.Context, app string, rt *Runtime) error {
	_, err := rt.RunCLI(ctx, "argocd", "wait", map[string]any{"app": app})
	return err
}

func ArgoRollbackActivity(ctx context.Context, app string, revision string, rt *Runtime) error {
	input := map[string]any{"app": app}
	if revision != "" {
		input["revision"] = revision
	}
	_, err := rt.RunCLI(ctx, "argocd", "rollback", input)
	return err
}

func HelmStatusActivity(ctx context.Context, release string, rt *Runtime) ([]byte, error) {
	return rt.RunCLI(ctx, "helm", "status", map[string]any{"release": release})
}

func HelmUpgradeActivity(ctx context.Context, release string, rt *Runtime) error {
	_, err := rt.RunCLI(ctx, "helm", "upgrade", map[string]any{"release": release})
	return err
}

func HelmRollbackActivity(ctx context.Context, release string, rt *Runtime) error {
	_, err := rt.RunCLI(ctx, "helm", "rollback", map[string]any{"release": release})
	return err
}

func KubectlScaleActivity(ctx context.Context, resource string, replicas int, rt *Runtime) error {
	_, err := rt.RunCLI(ctx, "kubectl", "scale", map[string]any{"resource": resource, "replicas": replicas})
	return err
}

func KubectlRolloutStatusActivity(ctx context.Context, resource string, rt *Runtime) error {
	_, err := rt.RunCLI(ctx, "kubectl", "rollout-status", map[string]any{"resource": resource})
	return err
}

func CreateGrafanaAnnotationActivity(ctx context.Context, payload []byte, rt *Runtime) error {
	_, err := runTool(ctx, rt, "grafana", "annotate", payload)
	return err
}

func DeleteGrafanaDashboardActivity(ctx context.Context, uid string, rt *Runtime) error {
	_, err := runTool(ctx, rt, "grafana", "dashboard_delete", map[string]any{"uid": uid})
	return err
}

func CreateLinearIssueActivity(ctx context.Context, payload []byte, rt *Runtime) (string, error) {
	out, err := runTool(ctx, rt, "linear", "create", payload)
	return string(out), err
}

func CreatePagerDutyIncidentActivity(ctx context.Context, payload []byte, rt *Runtime) (string, error) {
	out, err := runTool(ctx, rt, "pagerduty", "create", payload)
	return string(out), err
}

func CreateGitPullRequestActivity(ctx context.Context, payload []byte, rt *Runtime) (string, error) {
	out, err := runTool(ctx, rt, "github", "pr", payload)
	return string(out), err
}

func CreateGitLabMergeRequestActivity(ctx context.Context, payload []byte, rt *Runtime) (string, error) {
	out, err := runTool(ctx, rt, "gitlab", "mr", payload)
	return string(out), err
}

func PromRulesActivity(ctx context.Context, rt *Runtime) ([]byte, error) {
	return runTool(ctx, rt, "prometheus", "rules", map[string]any{})
}

func PromQueryRangeActivity(ctx context.Context, query, start, end, step string, rt *Runtime) ([]byte, error) {
	input := map[string]any{"query": query, "start": start, "end": end, "step": step}
	return runTool(ctx, rt, "prometheus", "query_range", input)
}

func TempoTraceByIDActivity(ctx context.Context, traceID string, rt *Runtime) ([]byte, error) {
	return runTool(ctx, rt, "tempo", "trace_by_id", map[string]any{"trace_id": traceID})
}

func VaultRenewActivity(ctx context.Context, leaseID string, rt *Runtime) error {
	_, err := runTool(ctx, rt, "vault", "renew", map[string]any{"lease_id": leaseID})
	return err
}

func VaultRevokeActivity(ctx context.Context, leaseID string, rt *Runtime) error {
	_, err := runTool(ctx, rt, "vault", "revoke", map[string]any{"lease_id": leaseID})
	return err
}

func runTool(ctx context.Context, rt *Runtime, tool, action string, payload any) ([]byte, error) {
	if rt == nil {
		return nil, errors.New("runtime required")
	}
	input, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	return rt.RunCLI(ctx, tool, action, input)
}

func decodePayload(payload any) (any, error) {
	switch v := payload.(type) {
	case nil:
		return nil, nil
	case []byte:
		if len(v) == 0 {
			return nil, nil
		}
		var out any
		if err := json.Unmarshal(v, &out); err != nil {
			return nil, err
		}
		return out, nil
	default:
		return payload, nil
	}
}
