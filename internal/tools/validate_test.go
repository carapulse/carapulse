package tools

import (
	"encoding/json"
	"testing"
)

func TestValidateExecuteRequestMissingTool(t *testing.T) {
	if _, err := validateExecuteRequest(ExecuteRequest{Action: "x"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateExecuteRequestMissingAction(t *testing.T) {
	if _, err := validateExecuteRequest(ExecuteRequest{Tool: "kubectl"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateExecuteRequestUnknownTool(t *testing.T) {
	if _, err := validateExecuteRequest(ExecuteRequest{Tool: "nope", Action: "x"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateKubectlScaleOK(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "svc", "replicas": 1}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateKubectlScaleBadReplicas(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "svc", "replicas": -1}}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateKubectlScaleMissingResource(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "scale", Input: map[string]any{"replicas": 1}}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateKubectlScaleMissingReplicas(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "scale", Input: map[string]any{"resource": "svc"}}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateKubectlScaleMissingInput(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "scale"}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateKubectlRolloutOK(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "rollout-status", Input: map[string]any{"resource": "svc"}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateKubectlRolloutMissingResource(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "rollout-status", Input: map[string]any{}}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateKubectlUnsupportedAction(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "bad", Input: map[string]any{}}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateHelmOK(t *testing.T) {
	req := ExecuteRequest{Tool: "helm", Action: "status", Input: map[string]any{"release": "rel"}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateHelmMissingRelease(t *testing.T) {
	req := ExecuteRequest{Tool: "helm", Action: "status", Input: map[string]any{}}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateHelmUpgradeOK(t *testing.T) {
	req := ExecuteRequest{Tool: "helm", Action: "upgrade", Input: map[string]any{"release": "rel"}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateHelmUpgradeValuesRefOK(t *testing.T) {
	req := ExecuteRequest{
		Tool:   "helm",
		Action: "upgrade",
		Input:  map[string]any{"release": "rel", "values_ref": map[string]any{"kind": "inline", "ref": "data"}},
	}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateHelmUpgradeValuesRefInvalid(t *testing.T) {
	req := ExecuteRequest{
		Tool:   "helm",
		Action: "upgrade",
		Input:  map[string]any{"release": "rel", "values_ref": map[string]any{"kind": "bad", "ref": "data"}},
	}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateHelmInputError(t *testing.T) {
	req := ExecuteRequest{Tool: "helm", Action: "status", Input: []byte("{")}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateHelmUnsupportedAction(t *testing.T) {
	req := ExecuteRequest{Tool: "helm", Action: "bad", Input: map[string]any{}}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateArgoOK(t *testing.T) {
	req := ExecuteRequest{Tool: "argocd", Action: "sync", Input: map[string]any{"app": "app"}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateArgoWaitOK(t *testing.T) {
	req := ExecuteRequest{Tool: "argocd", Action: "wait", Input: map[string]any{"app": "app"}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateArgoMissingApp(t *testing.T) {
	req := ExecuteRequest{Tool: "argocd", Action: "sync", Input: map[string]any{}}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateArgoInputError(t *testing.T) {
	req := ExecuteRequest{Tool: "argocd", Action: "sync"}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateArgoUnsupportedAction(t *testing.T) {
	req := ExecuteRequest{Tool: "argocd", Action: "bad", Input: map[string]any{}}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateArgoListOK(t *testing.T) {
	req := ExecuteRequest{Tool: "argocd", Action: "list", Input: map[string]any{}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateHelmListOK(t *testing.T) {
	req := ExecuteRequest{Tool: "helm", Action: "list", Input: map[string]any{}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateKubectlGetOK(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "get", Input: map[string]any{"resource": "deploy/app"}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidatePrometheusQueryRangeMissing(t *testing.T) {
	req := ExecuteRequest{Tool: "prometheus", Action: "query_range", Input: map[string]any{"query": "up"}}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateTempoTraceByIDMissing(t *testing.T) {
	req := ExecuteRequest{Tool: "tempo", Action: "trace_by_id", Input: map[string]any{}}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateGrafanaDashboardGetOK(t *testing.T) {
	req := ExecuteRequest{Tool: "grafana", Action: "dashboard_get", Input: map[string]any{"uid": "dash"}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateKubectlWatchOK(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "watch", Input: map[string]any{"resource": "pods", "timeout_seconds": 10}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateArgoProjectTokenCreateOK(t *testing.T) {
	req := ExecuteRequest{Tool: "argocd", Action: "project_token_create", Input: map[string]any{"project": "proj", "role": "role"}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateGitCommitMissingMessage(t *testing.T) {
	req := ExecuteRequest{Tool: "git", Action: "commit", Input: map[string]any{}}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateGitBranchOK(t *testing.T) {
	req := ExecuteRequest{Tool: "git", Action: "branch", Input: map[string]any{"name": "feat"}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateInputMapJSON(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"resource": "svc", "replicas": 1})
	req := ExecuteRequest{Tool: "kubectl", Action: "scale", Input: payload}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateInputMapJSONRawMessage(t *testing.T) {
	payload := json.RawMessage(`{"resource":"svc","replicas":1}`)
	req := ExecuteRequest{Tool: "kubectl", Action: "scale", Input: payload}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateInputMapJSONString(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "scale", Input: `{"resource":"svc","replicas":1}`}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateInputMapEmptyString(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "scale", Input: ""}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateContextRefStrictMissing(t *testing.T) {
	err := validateContextRefStrict(ContextRef{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateContextRefStrictOK(t *testing.T) {
	ctx := ContextRef{
		TenantID:      "t",
		Environment:   "prod",
		ClusterID:     "c",
		Namespace:     "ns",
		AWSAccountID:  "123",
		Region:        "us-east-1",
		ArgoCDProject: "proj",
		GrafanaOrgID:  "1",
	}
	if err := validateContextRefStrict(ctx); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateInputMapWrongType(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "scale", Input: 123}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateInputMapBadJSON(t *testing.T) {
	req := ExecuteRequest{Tool: "kubectl", Action: "scale", Input: []byte("{")}
	if _, err := validateExecuteRequest(req); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateNonCLIActionOK(t *testing.T) {
	req := ExecuteRequest{Tool: "prometheus", Action: "query", Input: map[string]any{"query": "up"}}
	if _, err := validateExecuteRequest(req); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestStringFieldNilMap(t *testing.T) {
	if got := stringField(nil, "key"); got != "" {
		t.Fatalf("expected empty")
	}
}

func TestIntFromAnyOK(t *testing.T) {
	if _, ok := intFromAnyOK(int64(2)); !ok {
		t.Fatalf("expected ok")
	}
	if _, ok := intFromAnyOK(float64(2)); !ok {
		t.Fatalf("expected ok")
	}
	if _, ok := intFromAnyOK("nope"); ok {
		t.Fatalf("expected false")
	}
}
