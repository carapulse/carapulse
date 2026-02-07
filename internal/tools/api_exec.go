package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

func (r *Router) ExecuteAPI(ctx context.Context, tool string, action string, input any, clients HTTPClients) ([]byte, error) {
	switch tool {
	case "prometheus":
		if clients.Prometheus == nil {
			return nil, ErrNoCLI
		}
		switch action {
		case "query":
			return clients.Prometheus.Do(ctx, "POST", "/api/v1/query", input)
		case "query_range":
			return clients.Prometheus.Do(ctx, "POST", "/api/v1/query_range", input)
		case "rules":
			return clients.Prometheus.Do(ctx, "GET", "/api/v1/rules", nil)
		case "query_exemplars":
			return clients.Prometheus.Do(ctx, "POST", "/api/v1/query_exemplars", input)
		default:
			return nil, ErrNoCLI
		}
	case "alertmanager":
		if clients.Alertmanager == nil {
			return nil, ErrNoCLI
		}
		switch action {
		case "alerts_list":
			return clients.Alertmanager.Do(ctx, "GET", "/api/v2/alerts", nil)
		default:
			return nil, ErrNoCLI
		}
	case "thanos":
		if clients.Thanos == nil {
			return nil, ErrNoCLI
		}
		switch action {
		case "query":
			path := "/api/v1/query"
			if q := thanosQueryParams(input); q != "" {
				path += "?" + q
			}
			return clients.Thanos.Do(ctx, "POST", path, input)
		case "query_range":
			path := "/api/v1/query_range"
			if q := thanosQueryParams(input); q != "" {
				path += "?" + q
			}
			return clients.Thanos.Do(ctx, "POST", path, input)
		default:
			return nil, ErrNoCLI
		}
	case "grafana":
		if clients.Grafana == nil {
			return nil, ErrNoCLI
		}
		switch action {
		case "annotate":
			return clients.Grafana.Do(ctx, "POST", "/api/annotations", input)
		case "dashboard_get":
			uid := stringFieldFromInput(input, "uid")
			if uid == "" {
				return nil, ErrNoCLI
			}
			return clients.Grafana.Do(ctx, "GET", "/api/dashboards/uid/"+url.PathEscape(uid), nil)
		case "dashboard_upsert":
			return clients.Grafana.Do(ctx, "POST", "/api/dashboards/db", input)
		case "dashboard_list":
			path := "/api/search"
			if q := grafanaSearchParams(input); q != "" {
				path += "?" + q
			}
			return clients.Grafana.Do(ctx, "GET", path, nil)
		case "dashboard_delete":
			uid := stringFieldFromInput(input, "uid")
			if uid == "" {
				return nil, ErrNoCLI
			}
			return clients.Grafana.Do(ctx, "DELETE", "/api/dashboards/uid/"+url.PathEscape(uid), nil)
		default:
			return nil, ErrNoCLI
		}
	case "tempo":
		if clients.Tempo == nil {
			return nil, ErrNoCLI
		}
		switch action {
		case "query", "traceql":
			return clients.Tempo.Do(ctx, "POST", "/api/search", input)
		case "trace_by_id":
			traceID := stringFieldFromInput(input, "trace_id")
			if traceID == "" {
				return nil, ErrNoCLI
			}
			return clients.Tempo.Do(ctx, "GET", "/api/v2/traces/"+url.PathEscape(traceID), nil)
		default:
			return nil, ErrNoCLI
		}
	case "vault":
		if clients.Vault == nil {
			return nil, ErrNoCLI
		}
		switch action {
		case "login_k8s":
			return clients.Vault.Do(ctx, "POST", "/v1/auth/kubernetes/login", input)
		case "login_approle":
			return clients.Vault.Do(ctx, "POST", "/v1/auth/approle/login", input)
		case "renew":
			return clients.Vault.Do(ctx, "POST", "/v1/sys/leases/renew", input)
		case "revoke":
			return clients.Vault.Do(ctx, "POST", "/v1/sys/leases/revoke", input)
		case "health":
			return clients.Vault.Do(ctx, "GET", "/v1/sys/health", nil)
		case "audit_enable":
			path := stringFieldFromInput(input, "path")
			if path == "" {
				path = "file"
			}
			payload := map[string]any{"type": stringFieldFromInput(input, "type")}
			if payload["type"] == "" {
				payload["type"] = "file"
			}
			if filePath := stringFieldFromInput(input, "path"); filePath != "" {
				payload["options"] = map[string]any{"file_path": filePath}
			}
			return clients.Vault.Do(ctx, "POST", "/v1/sys/audit/"+url.PathEscape(path), payload)
		case "token_renew":
			return clients.Vault.Do(ctx, "POST", "/v1/auth/token/renew-self", map[string]any{})
		default:
			return nil, ErrNoCLI
		}
	case "boundary":
		if clients.Boundary == nil {
			return nil, ErrNoCLI
		}
		switch action {
		case "authenticate":
			return clients.Boundary.Do(ctx, "POST", "/v1/auth-methods/login", input)
		case "session_open":
			return clients.Boundary.Do(ctx, "POST", "/v1/sessions", input)
		case "session_close":
			id := stringFieldFromInput(input, "session_id")
			if id == "" {
				return nil, ErrNoCLI
			}
			return clients.Boundary.Do(ctx, "POST", "/v1/sessions/"+url.PathEscape(id)+":cancel", nil)
		default:
			return nil, ErrNoCLI
		}
	case "argocd":
		if clients.ArgoCD == nil {
			return nil, ErrNoCLI
		}
		switch action {
		case "status":
			app := stringFieldFromInput(input, "app")
			if app == "" {
				return nil, ErrNoCLI
			}
			return clients.ArgoCD.Do(ctx, "GET", "/api/v1/applications/"+url.PathEscape(app), nil)
		default:
			return nil, ErrNoCLI
		}
	case "aws":
		if clients.AWS == nil {
			return nil, ErrNoCLI
		}
		switch action {
		case "cloudtrail_lookup_events", "cloudtrail-lookup-events":
			return clients.AWS.Do(ctx, "POST", "/cloudtrail/lookup", input)
		case "cloudwatch_get_metric_data", "cloudwatch-get-metric-data":
			return clients.AWS.Do(ctx, "POST", "/cloudwatch/metric-data", input)
		default:
			return nil, ErrNoCLI
		}
	case "linear":
		if clients.Linear == nil {
			return nil, ErrNoCLI
		}
		return clients.Linear.Do(ctx, "POST", "/graphql", input)
	case "pagerduty":
		if clients.PagerDuty == nil {
			return nil, ErrNoCLI
		}
		return clients.PagerDuty.Do(ctx, "POST", "/v2/enqueue", input)
	default:
		return nil, ErrNoCLI
	}
}

func stringFieldFromInput(input any, key string) string {
	m, err := inputMap(input)
	if err != nil {
		return ""
	}
	return stringField(m, key)
}

func thanosQueryParams(input any) string {
	m, err := inputMap(input)
	if err != nil {
		return ""
	}
	values := url.Values{}
	if dedup, ok := m["dedup"].(bool); ok {
		values.Set("dedup", fmt.Sprintf("%t", dedup))
	}
	if msr, ok := m["max_source_resolution"].(string); ok && strings.TrimSpace(msr) != "" {
		values.Set("max_source_resolution", strings.TrimSpace(msr))
	}
	return values.Encode()
}

func grafanaSearchParams(input any) string {
	m, err := inputMap(input)
	if err != nil {
		return ""
	}
	values := url.Values{}
	if query, ok := m["query"].(string); ok && strings.TrimSpace(query) != "" {
		values.Set("query", strings.TrimSpace(query))
	}
	if folders, ok := m["folder_ids"].([]any); ok {
		for _, folder := range folders {
			if id, ok := intFromAnyOK(folder); ok {
				values.Add("folderIds", fmt.Sprintf("%d", id))
			}
		}
	}
	return values.Encode()
}
