package workflows

import (
	"errors"
	"fmt"
	"strings"
)

func BuildWorkflowSteps(name string, input map[string]any) (string, []PlanStep, error) {
	switch name {
	case "gitops_deploy":
		return buildGitOpsDeploySteps(input)
	case "helm_release":
		return buildHelmReleaseSteps(input)
	case "scale_service":
		return buildScaleServiceSteps(input)
	case "incident_remediation":
		return buildIncidentRemediationSteps(input)
	case "secret_rotation":
		return buildSecretRotationSteps(input)
	default:
		return "", nil, errors.New("unknown workflow")
	}
}

func buildGitOpsDeploySteps(input map[string]any) (string, []PlanStep, error) {
	app := stringValue(input, "argocd_app", "app")
	if app == "" {
		return "", nil, errors.New("argocd_app required")
	}
	service := stringValue(input, "service")
	revision := stringValue(input, "revision")
	promql := stringValue(input, "promql")
	text := stringValue(input, "annotation")
	if text == "" {
		if service != "" {
			text = "GitOps deploy " + service
		} else {
			text = "GitOps deploy " + app
		}
	}
	steps := []PlanStep{
		{Action: "sync-dry-run", Tool: "argocd", Input: map[string]any{"app": app}},
		{Action: "sync-preview", Tool: "argocd", Input: map[string]any{"app": app}},
		{
			Action: "sync",
			Tool:   "argocd",
			Input:  map[string]any{"app": app},
			Rollback: rollbackStep("argocd", "rollback", map[string]any{
				"app":      app,
				"revision": revision,
			}, revision != ""),
		},
		{Stage: "verify", Action: "wait", Tool: "argocd", Input: map[string]any{"app": app}},
	}
	if promql != "" {
		steps = append(steps, PlanStep{Stage: "verify", Action: "query", Tool: "prometheus", Input: map[string]any{"query": promql}})
	}
	steps = append(steps, PlanStep{Action: "annotate", Tool: "grafana", Input: map[string]any{"text": text, "tags": []string{"gitops", "deploy"}}})
	summary := "GitOps deploy"
	if service != "" {
		summary += " " + service
	}
	return summary, steps, nil
}

func buildHelmReleaseSteps(input map[string]any) (string, []PlanStep, error) {
	release := stringValue(input, "release")
	if release == "" {
		return "", nil, errors.New("release required")
	}
	chart := stringValue(input, "chart")
	strategy := stringValue(input, "strategy")
	namespace := stringValue(input, "namespace")
	promql := stringValue(input, "promql")
	text := stringValue(input, "annotation")
	valuesRef := input["values_ref"]
	if text == "" {
		text = "Helm release " + release
	}
	steps := []PlanStep{
		{Action: "status", Tool: "helm", Input: map[string]any{"release": release, "namespace": namespace}},
	}
	if strings.EqualFold(strategy, "rollback") {
		steps = append(steps, PlanStep{Action: "rollback", Tool: "helm", Input: map[string]any{"release": release, "namespace": namespace}})
	} else {
		upgradeInput := map[string]any{"release": release, "namespace": namespace}
		if chart != "" {
			upgradeInput["chart"] = chart
		}
		if valuesRef != nil {
			upgradeInput["values_ref"] = valuesRef
		}
		steps = append(steps, PlanStep{
			Action: "upgrade",
			Tool:   "helm",
			Input:  upgradeInput,
			Rollback: rollbackStep("helm", "rollback", map[string]any{
				"release":   release,
				"namespace": namespace,
			}, true),
		})
	}
	steps = append(steps, PlanStep{Stage: "verify", Action: "status", Tool: "helm", Input: map[string]any{"release": release, "namespace": namespace}})
	if rollout := stringValue(input, "rollout_resource"); rollout != "" {
		steps = append(steps, PlanStep{Stage: "verify", Action: "rollout-status", Tool: "kubectl", Input: map[string]any{"resource": rollout}})
	}
	if promql != "" {
		steps = append(steps, PlanStep{Stage: "verify", Action: "query", Tool: "prometheus", Input: map[string]any{"query": promql}})
	}
	steps = append(steps, PlanStep{Action: "annotate", Tool: "grafana", Input: map[string]any{"text": text, "tags": []string{"helm", "deploy"}}})
	summary := "Helm release " + release
	return summary, steps, nil
}

func buildScaleServiceSteps(input map[string]any) (string, []PlanStep, error) {
	resource := stringValue(input, "resource", "service")
	if resource == "" {
		return "", nil, errors.New("resource required")
	}
	replicas, ok := intValue(input, "replicas")
	if !ok {
		return "", nil, errors.New("replicas required")
	}
	current, _ := intValue(input, "current_replicas")
	previous, _ := intValue(input, "previous_replicas")
	promql := stringValue(input, "promql")
	text := stringValue(input, "annotation")
	if text == "" {
		text = fmt.Sprintf("Scale %s to %d", resource, replicas)
	}
	scaleInput := map[string]any{"resource": resource, "replicas": replicas}
	if current > 0 {
		scaleInput["current_replicas"] = current
	}
	steps := []PlanStep{
		{
			Action: "scale",
			Tool:   "kubectl",
			Input:  scaleInput,
			Rollback: rollbackStep("kubectl", "scale", map[string]any{
				"resource": resource,
				"replicas": previous,
			}, previous > 0),
		},
		{Stage: "verify", Action: "rollout-status", Tool: "kubectl", Input: map[string]any{"resource": resource}},
	}
	if promql != "" {
		steps = append(steps, PlanStep{Stage: "verify", Action: "query", Tool: "prometheus", Input: map[string]any{"query": promql}})
	}
	steps = append(steps, PlanStep{Action: "annotate", Tool: "grafana", Input: map[string]any{"text": text, "tags": []string{"scale"}}})
	summary := fmt.Sprintf("Scale %s to %d", resource, replicas)
	return summary, steps, nil
}

func buildIncidentRemediationSteps(input map[string]any) (string, []PlanStep, error) {
	service := stringValue(input, "service")
	promql := stringValue(input, "promql")
	traceID := stringValue(input, "trace_id")
	traceQL := stringValue(input, "traceql")
	text := stringValue(input, "annotation")
	if text == "" {
		if service != "" {
			text = "Incident remediation " + service
		} else {
			text = "Incident remediation"
		}
	}
	steps := []PlanStep{
		{Stage: "verify", Action: "rules", Tool: "prometheus", Input: map[string]any{}},
	}
	if promql != "" {
		steps = append(steps, PlanStep{Stage: "verify", Action: "query", Tool: "prometheus", Input: map[string]any{"query": promql}})
	}
	if traceQL != "" {
		steps = append(steps, PlanStep{Stage: "verify", Action: "traceql", Tool: "tempo", Input: map[string]any{"query": traceQL}})
	}
	if traceID != "" {
		steps = append(steps, PlanStep{Stage: "verify", Action: "trace_by_id", Tool: "tempo", Input: map[string]any{"trace_id": traceID}})
	}
	if remediation, ok := input["remediation"].(map[string]any); ok {
		tool := stringValue(remediation, "tool")
		action := stringValue(remediation, "action")
		stepInput := remediation["input"]
		if tool != "" && action != "" {
			steps = append(steps, PlanStep{
				Action:   action,
				Tool:     tool,
				Input:    stepInput,
				Rollback: remediation["rollback"],
			})
		}
	}
	steps = append(steps, PlanStep{Action: "annotate", Tool: "grafana", Input: map[string]any{"text": text, "tags": []string{"incident"}}})
	summary := "Incident remediation"
	if service != "" {
		summary += " " + service
	}
	return summary, steps, nil
}

func buildSecretRotationSteps(input map[string]any) (string, []PlanStep, error) {
	secret := stringValue(input, "secret_path")
	if secret == "" {
		return "", nil, errors.New("secret_path required")
	}
	argocdApp := stringValue(input, "argocd_app", "app")
	promql := stringValue(input, "promql")
	text := stringValue(input, "annotation")
	if text == "" {
		text = "Secret rotation " + secret
	}
	steps := []PlanStep{
		{Action: "renew", Tool: "vault", Input: map[string]any{"lease_id": secret}},
		{Action: "revoke", Tool: "vault", Input: map[string]any{"lease_id": secret}},
	}
	if argocdApp != "" {
		steps = append(steps, PlanStep{Action: "sync", Tool: "argocd", Input: map[string]any{"app": argocdApp}})
	}
	if promql != "" {
		steps = append(steps, PlanStep{Stage: "verify", Action: "query", Tool: "prometheus", Input: map[string]any{"query": promql}})
	}
	steps = append(steps, PlanStep{Action: "annotate", Tool: "grafana", Input: map[string]any{"text": text, "tags": []string{"secret", "rotation"}}})
	summary := "Secret rotation"
	if secret != "" {
		summary += " " + secret
	}
	return summary, steps, nil
}

func stringValue(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func intValue(m map[string]any, key string) (int, bool) {
	val, ok := m[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func rollbackStep(tool, action string, input map[string]any, enabled bool) any {
	if !enabled {
		return nil
	}
	return map[string]any{
		"tool":   tool,
		"action": action,
		"input":  input,
	}
}
