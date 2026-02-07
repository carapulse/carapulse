package web

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"carapulse/internal/tools"
)

type DiagnosticEvidence struct {
	Type      string `json:"type"`
	Query     string `json:"query"`
	ResultRef string `json:"result_ref"`
	Link      string `json:"link"`
}

type DiagnosticsCollector interface {
	Collect(ctx context.Context, ctxRef ContextRef, intent string, constraints any) ([]DiagnosticEvidence, error)
}

type ObjectStore interface {
	Put(ctx context.Context, key string, data []byte) (string, error)
	Presign(ctx context.Context, key string, ttl time.Duration) (string, error)
}

type ToolDiagnostics struct {
	Router     *tools.RouterClient
	Store      ObjectStore
	PresignTTL time.Duration
	Now        func() time.Time
}

func (d *ToolDiagnostics) Collect(ctx context.Context, ctxRef ContextRef, intent string, constraints any) ([]DiagnosticEvidence, error) {
	if d.Router == nil {
		return nil, nil
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	promQueries, traceQueries, traceIDs := diagnosticHintsFromConstraints(constraints, intent)
	now := d.Now().UTC()
	start := now.Add(-15 * time.Minute).Format(time.RFC3339)
	end := now.Format(time.RFC3339)
	step := "60s"

	var out []DiagnosticEvidence
	routes := []struct {
		tool   string
		action string
		input  map[string]any
		etype  string
		query  string
	}{
		{tool: "prometheus", action: "rules", input: map[string]any{}, etype: "promql", query: "rules"},
	}
	for _, query := range promQueries {
		routes = append(routes, struct {
			tool   string
			action string
			input  map[string]any
			etype  string
			query  string
		}{tool: "prometheus", action: "query_range", input: map[string]any{"query": query, "start": start, "end": end, "step": step}, etype: "promql", query: query})
		routes = append(routes, struct {
			tool   string
			action string
			input  map[string]any
			etype  string
			query  string
		}{tool: "prometheus", action: "query_exemplars", input: map[string]any{"query": query, "start": start, "end": end}, etype: "promql", query: query})
	}
	for _, query := range traceQueries {
		routes = append(routes, struct {
			tool   string
			action string
			input  map[string]any
			etype  string
			query  string
		}{tool: "tempo", action: "traceql", input: map[string]any{"query": query}, etype: "traceql", query: query})
	}
	for _, traceID := range traceIDs {
		routes = append(routes, struct {
			tool   string
			action string
			input  map[string]any
			etype  string
			query  string
		}{tool: "tempo", action: "trace_by_id", input: map[string]any{"trace_id": traceID}, etype: "traceql", query: traceID})
	}
	for _, route := range routes {
		resp, err := d.Router.Execute(ctx, tools.ExecuteRequest{
			Tool:    route.tool,
			Action:  route.action,
			Input:   route.input,
			Context: toToolContext(ctxRef),
		})
		if err != nil {
			continue
		}
		resultRef, link := d.storeDiagnostic(ctx, route.tool, route.action, resp.Output)
		out = append(out, DiagnosticEvidence{
			Type:      route.etype,
			Query:     route.query,
			ResultRef: resultRef,
			Link:      link,
		})
	}
	return out, nil
}

func (d *ToolDiagnostics) storeDiagnostic(ctx context.Context, tool, action string, output []byte) (string, string) {
	if d.Store == nil || len(output) == 0 {
		return "", ""
	}
	ts := d.Now().UTC().Format("20060102T150405Z")
	key := fmt.Sprintf("diagnostics/%s/%s-%s.json", ts, tool, action)
	ref, err := d.Store.Put(ctx, key, output)
	if err != nil {
		return "", ""
	}
	link := ""
	if signed, err := d.Store.Presign(ctx, ref, d.PresignTTL); err == nil {
		link = signed
	}
	return ref, link
}

func queryFromIntent(intent string) string {
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

func diagnosticHintsFromConstraints(constraints any, intent string) ([]string, []string, []string) {
	prom := []string{}
	trace := []string{}
	traceIDs := []string{}
	if c, ok := constraints.(map[string]any); ok {
		if hints, ok := c["diagnostic_hints"].(map[string]any); ok {
			prom = stringSliceFromAny(hints["promql"])
			trace = stringSliceFromAny(hints["traceql"])
			traceIDs = stringSliceFromAny(hints["trace_ids"])
		} else {
			prom = stringSliceFromAny(c["promql"])
			trace = stringSliceFromAny(c["traceql"])
			traceIDs = stringSliceFromAny(c["trace_ids"])
		}
	}
	if len(prom) == 0 {
		prom = []string{queryFromIntent(intent)}
	}
	return uniqueStrings(prom), uniqueStrings(trace), uniqueStrings(traceIDs)
}

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func toToolContext(ctx ContextRef) tools.ContextRef {
	return tools.ContextRef{
		TenantID:      ctx.TenantID,
		Environment:   ctx.Environment,
		ClusterID:     ctx.ClusterID,
		Namespace:     ctx.Namespace,
		AWSAccountID:  ctx.AWSAccountID,
		Region:        ctx.Region,
		ArgoCDProject: ctx.ArgoCDProject,
		GrafanaOrgID:  ctx.GrafanaOrgID,
	}
}

func diagnosticsFromJSON(data []byte) ([]DiagnosticEvidence, error) {
	var items []DiagnosticEvidence
	if len(data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}
