package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "carapulse",
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests by method, path, and status code.",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "carapulse",
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request latency in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path"})

	ToolExecutionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "carapulse",
		Name:      "tool_executions_total",
		Help:      "Total tool executions by tool, action, and outcome.",
	}, []string{"tool", "action", "outcome"})

	ToolExecutionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "carapulse",
		Name:      "tool_execution_duration_seconds",
		Help:      "Tool execution latency in seconds.",
		Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
	}, []string{"tool", "action"})

	WorkflowExecutionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "carapulse",
		Name:      "workflow_executions_total",
		Help:      "Total workflow executions by workflow type and outcome.",
	}, []string{"workflow", "outcome"})

	PolicyDecisionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "carapulse",
		Name:      "policy_decisions_total",
		Help:      "Total OPA policy decisions by decision type.",
	}, []string{"decision"})

	ApprovalsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "carapulse",
		Name:      "approvals_total",
		Help:      "Total approvals by status (approved, denied, timeout).",
	}, []string{"status"})

	ActiveWebSocketConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "carapulse",
		Name:      "active_websocket_connections",
		Help:      "Number of active WebSocket connections.",
	})

	ActiveSSEConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "carapulse",
		Name:      "active_sse_connections",
		Help:      "Number of active SSE connections.",
	})
)

// Handler returns an http.Handler that serves the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}

// Middleware wraps an http.Handler to record request metrics.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()

		path := normalizePath(r.URL.Path)
		HTTPRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rw.statusCode)).Inc()
		HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// normalizePath buckets URL paths to avoid high cardinality.
// It keeps the first two path segments and replaces the rest with a placeholder.
func normalizePath(p string) string {
	if p == "" || p == "/" {
		return "/"
	}
	// Keep known static paths
	switch {
	case p == "/healthz" || p == "/readyz" || p == "/metrics":
		return p
	}
	// For API paths like /v1/plans/abc123, keep /v1/plans
	segments := 0
	for i := 1; i < len(p); i++ {
		if p[i] == '/' {
			segments++
			if segments >= 2 {
				return p[:i]
			}
		}
	}
	return p
}
