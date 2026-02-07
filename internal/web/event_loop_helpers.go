package web

import (
	"encoding/json"
	"strings"

	ctxmodel "carapulse/internal/context"
)

func ensureVerifySteps(steps []planStepDraft, diagnostics []DiagnosticEvidence, intent string) []planStepDraft {
	if len(steps) == 0 {
		return steps
	}
	if hasVerifyStage(steps) {
		return steps
	}
	verify := defaultVerifySteps(diagnostics, intent)
	if len(verify) == 0 {
		return steps
	}
	return append(steps, verify...)
}

func hasVerifyStage(steps []planStepDraft) bool {
	for _, step := range steps {
		if strings.EqualFold(strings.TrimSpace(step.Stage), "verify") {
			return true
		}
	}
	return false
}

func defaultVerifySteps(diagnostics []DiagnosticEvidence, intent string) []planStepDraft {
	for _, diag := range diagnostics {
		query := strings.TrimSpace(diag.Query)
		switch strings.ToLower(strings.TrimSpace(diag.Type)) {
		case "promql":
			if query == "" {
				query = defaultQueryFromIntent(intent)
			}
			if query != "" {
				return []planStepDraft{{Stage: "verify", Tool: "prometheus", Action: "query", Input: rawJSON(map[string]any{"query": query})}}
			}
		case "traceql":
			if query != "" {
				return []planStepDraft{{Stage: "verify", Tool: "tempo", Action: "traceql", Input: rawJSON(map[string]any{"query": query})}}
			}
		}
	}
	query := defaultQueryFromIntent(intent)
	if query == "" {
		return nil
	}
	return []planStepDraft{{Stage: "verify", Tool: "prometheus", Action: "query", Input: rawJSON(map[string]any{"query": query})}}
}

func defaultQueryFromIntent(intent string) string {
	intent = strings.ToLower(strings.TrimSpace(intent))
	if intent == "" {
		return "up"
	}
	if strings.Contains(intent, "latency") {
		return "histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))"
	}
	if strings.Contains(intent, "error") {
		return "sum(rate(http_requests_total{status=~\"5..\"}[5m]))"
	}
	return "up"
}

func serviceFromHook(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if ctxRaw, ok := payload["context"].(map[string]any); ok {
		if svc := stringFromMap(ctxRaw, "service", "service_name", "app", "application"); svc != "" {
			return svc
		}
	}
	if labels := extractAlertLabels(payload); len(labels) > 0 {
		if svc, ok := labels["service"]; ok {
			if s, ok := svc.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
		for _, key := range []string{"service_name", "app", "application", "job"} {
			if v, ok := labels[key]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return s
				}
			}
		}
	}
	return ""
}

func rawJSON(payload map[string]any) json.RawMessage {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return json.RawMessage(data)
}

func diagnosticHintsFromGraph(graph any) map[string]any {
	if graph == nil {
		return nil
	}
	sg, ok := graph.(ctxmodel.ServiceGraph)
	if !ok {
		var decoded ctxmodel.ServiceGraph
		if data, err := json.Marshal(graph); err == nil {
			if err := json.Unmarshal(data, &decoded); err == nil {
				sg = decoded
				ok = true
			}
		}
	}
	if !ok {
		return nil
	}
	prom := []string{}
	trace := []string{}
	for _, node := range sg.Nodes {
		switch strings.ToLower(strings.TrimSpace(node.Kind)) {
		case "prometheus.query":
			if q := strings.TrimSpace(node.Name); q != "" {
				prom = append(prom, q)
			}
		case "tempo.query":
			if q := strings.TrimSpace(node.Name); q != "" {
				trace = append(trace, q)
			}
		}
	}
	if len(prom) == 0 && len(trace) == 0 {
		return nil
	}
	out := map[string]any{}
	if len(prom) > 0 {
		out["promql"] = prom
	}
	if len(trace) > 0 {
		out["traceql"] = trace
	}
	return out
}
