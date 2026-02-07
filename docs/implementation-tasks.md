# Carapulse Implementation Tasks

Exhaustive task list for making Carapulse fully operational. Organized by category, prioritized within each section. Each task is independent unless noted.

---

## 1. Dockerfiles & Container Images

### 1.1 Create multi-stage Dockerfile for gateway
- **File:** `docker/gateway.Dockerfile`
- **Build stage:** `golang:1.23-alpine`, build `./cmd/gateway`
- **Runtime stage:** `alpine:latest` (minimal — no CLI tools needed)
- **Expose:** Port 8080 (HTTP), 8082 (WebSocket)
- **Entrypoint:** `/usr/local/bin/gateway -config /etc/carapulse/config.json`
- **Config:** Mount as volume at `/etc/carapulse/`
- **Notes:** Gateway is pure Go HTTP — no external CLI deps at runtime. Needs ca-certificates for TLS to OPA/LLM/Linear.

### 1.2 Create multi-stage Dockerfile for tool-router
- **File:** `docker/tool-router.Dockerfile`
- **Build stage:** `golang:1.23-alpine`, build `./cmd/tool-router`
- **Runtime stage:** `debian:bookworm-slim` (needs CLI tools)
- **Install CLI tools:** `kubectl`, `helm`, `argocd`, `aws-cli`, `vault`, `boundary`, `git`, `gh`, `glab`
- **Expose:** Port 8081
- **Volume mounts:** Docker socket (`/var/run/docker.sock`) for sandbox execution
- **Entrypoint:** `/usr/local/bin/tool-router -config /etc/carapulse/config.json`
- **Notes:** Heaviest image — all DevOps CLI tools must be present. Consider a separate "tools" base image.

### 1.3 Create multi-stage Dockerfile for orchestrator
- **File:** `docker/orchestrator.Dockerfile`
- **Build stage:** `golang:1.23-alpine`, build `./cmd/orchestrator`
- **Runtime stage:** `debian:bookworm-slim`
- **Install:** Docker CLI (for sandbox support), `vault` CLI, `boundary` CLI
- **Volume mounts:** Docker socket, workspace dir at `storage.workspace_dir`
- **Entrypoint:** `/usr/local/bin/orchestrator -config /etc/carapulse/config.json`
- **Notes:** Needs Docker socket access for sandbox-exec. No HTTP port — connects outbound to Temporal.

### 1.4 Create multi-stage Dockerfile for agent
- **File:** `docker/agent.Dockerfile`
- **Build stage:** `golang:1.23-alpine`, build `./cmd/agent`
- **Runtime stage:** `alpine:latest` (minimal)
- **Expose:** Port 8090
- **Entrypoint:** `/usr/local/bin/agent -config /etc/carapulse/config.json`
- **Notes:** Lightweight — only makes HTTP calls to gateway.

### 1.5 Create Dockerfile for sandbox-exec base image
- **File:** `docker/sandbox.Dockerfile`
- **Base:** `debian:bookworm-slim`
- **Install:** `kubectl`, `helm`, `argocd`, `aws-cli`, `vault`, `boundary`, `git`, `gh`, `glab`, `curl`, `jq`
- **Purpose:** Base image used by `sandbox.image` config field for sandboxed tool execution
- **Security:** Non-root user, minimal packages, no shell history

### 1.6 Create CLI tools base image
- **Status:** Deferred
- **Purpose:** Shared base for tool-router/orchestrator/sandbox images to avoid duplication
- **Note:** Current Dockerfiles inline-install tools; add this later if build reuse becomes painful.

### 1.7 Add .dockerignore
- **File:** `.dockerignore`
- **Exclude:** `.git/`, `e2e/workspace/`, `*.md`, `docs/`, `scripts/`, `**/*_test.go`

---

## 2. Database Migration Runner

### 2.1 Add goose dependency
- **Command:** `go get github.com/pressly/goose/v3`
- **Why goose:** Pure Go, supports embedded SQL files, supports versioned migrations, lightweight

### 2.2 Create migration CLI command
- **File:** `cmd/migrate/main.go`
- **Flags:** `-dsn` (postgres DSN), `-dir` (migrations dir, default `./migrations`), `-action` (up/down/status/version/redo)
- **Implementation:** Use goose with `database/sql` + `lib/pq` driver
- **Example usage:** `go run ./cmd/migrate -dsn "postgres://..." -action up`

### 2.3 Embed migrations in binary (optional)
- **File:** `migrations/embed.go`
- **Use `//go:embed *.sql`** to embed migration files
- **Benefit:** Single binary deployment without needing migration files alongside

### 2.4 Add missing migration 0017
- **Gap:** `0017_*.sql` is missing from sequence (jumps 0016→0018)
- **Action:** Investigate if intentional. If not, add placeholder `0017_placeholder.sql` with a comment, or renumber 0018-0020.

### 2.5 Update e2e-up.sh to run migrations
- **File:** `scripts/e2e-up.sh`
- **Add after postgres healthcheck:** `go run ./cmd/migrate -dsn "postgres://carapulse:carapulse@127.0.0.1:5432/carapulse?sslmode=disable" -action up`
- **Ensure:** Wait for postgres readiness before running migrations

### 2.6 Add migration step to Dockerfiles
- **Option A:** Init container in Kubernetes that runs `migrate -action up` before services start
- **Option B:** Add migration check to gateway/orchestrator startup (run migrations if `--migrate` flag passed)
- **Recommended:** Option A for production, Option B for development

---

## 3. Health Endpoints

### 3.1 Add /healthz and /readyz to gateway
- **File:** `internal/web/http.go`
- **Routes:** `GET /healthz` (liveness), `GET /readyz` (readiness)
- **Liveness:** Always return 200 (proves process is running and HTTP server is listening)
- **Readiness checks:**
  - PostgreSQL: `SELECT 1` on DB connection
  - Temporal client: `client.CheckHealth()` if available
- **Response format:** `{"status": "ok"}` or `{"status": "unavailable", "checks": {"db": "error message"}}`
- **Do NOT wrap in AuthMiddleware** — health endpoints must be unauthenticated

### 3.2 Add /healthz and /readyz to tool-router
- **File:** `cmd/tool-router/main.go` (add routes to mux)
- **Liveness:** Return 200
- **Readiness checks:**
  - Sandbox runtime available: `docker version` or `podman version` succeeds (if sandbox enabled)
  - OPA reachable: HTTP GET to `policy.opa_url` (if configured)

### 3.3 Add /healthz and /readyz to orchestrator
- **Challenge:** Orchestrator has no HTTP server — it's a Temporal worker only
- **Option A:** Add a minimal HTTP server on a configurable port (e.g., `:8085`) just for health
- **Option B:** Use Temporal worker's built-in health (check if `worker.Run()` hasn't returned)
- **Recommended:** Option A — add `orchestrator.health_addr` config field, start `http.ListenAndServe` on it

### 3.4 Add /healthz and /readyz to agent
- **File:** `cmd/agent/main.go`
- **Liveness:** Return 200
- **Readiness:** Gateway reachable (HTTP GET to `chatops.gateway_url`)

### 3.5 Add goroutine health tracking to gateway
- **File:** `internal/web/http.go` or new `internal/web/health.go`
- **Track:** Whether each background goroutine (scheduler, approval watcher, alert poller, context service) is still running
- **Implementation:** Use atomic booleans or channels; set to unhealthy if goroutine exits unexpectedly
- **Report in /readyz:** Include goroutine status in readiness checks

---

## 4. Graceful Shutdown

### 4.1 Add signal handling to gateway
- **File:** `cmd/gateway/main.go`
- **Implementation:**
  1. Create root context: `ctx, cancel := context.WithCancel(context.Background())`
  2. Listen for signals: `signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)`
  3. Pass `ctx` (not `context.Background()`) to all goroutines: Scheduler.Run(ctx), ApprovalWatcher.Run(ctx), AlertPoller.Run(ctx), ContextService.Start(ctx)
  4. Replace `http.ListenAndServe` with `*http.Server{}.Shutdown(ctx)`
  5. Use `sync.WaitGroup` to track all goroutines
  6. On signal: cancel context → wait for goroutines (with timeout) → close DB → close Temporal client
- **All goroutines already check `<-ctx.Done()`** — just need a cancellable context

### 4.2 Add signal handling to tool-router
- **File:** `cmd/tool-router/main.go`
- **Goroutines to cancel:**
  - Vault health check ticker (line 148-156)
  - Vault token renewal ticker (line 160-169)
- **Changes needed:**
  - Wrap tickers in `select { case <-ctx.Done(): return; case <-ticker.C: ... }`
  - Use `*http.Server{}.Shutdown(ctx)` for HTTP server
  - Defer: Boundary session close, Vault cleanup

### 4.3 Add signal handling to orchestrator
- **File:** `cmd/orchestrator/main.go`
- **Already has:** `worker.Run(worker.InterruptCh())` — Temporal handles SIGTERM
- **Missing:** Vault health/renewal tickers (same pattern as tool-router)
- **Changes:** Wire cancellable context to Vault tickers, ensure Boundary cleanup runs

### 4.4 Add signal handling to agent
- **File:** `cmd/agent/main.go`
- **Simple:** Replace `http.ListenAndServe` with `*http.Server{}.Shutdown(ctx)` + signal listener

### 4.5 Add graceful shutdown for canvas HTTP server
- **File:** `cmd/gateway/main.go` (line 423)
- **Current:** `go func() { listen(cfg.Gateway.CanvasAddr, canvasMux) }()` — detached, no shutdown
- **Change:** Create `*http.Server{}`, track it, call `Shutdown()` on signal

### 4.6 Add shutdown timeout
- **All services:** After cancelling context, wait up to 30 seconds for goroutines to drain, then force exit
- **Pattern:** `time.AfterFunc(30*time.Second, func() { os.Exit(1) })`

---

## 5. OPA Rego Policy Rules

### 5.1 Create base policy rule file
- **File:** `policies/policy/assistant/v1/base.rego`
- **Package:** `policy.assistant.v1`
- **Default decision:** `deny` (deny by default, explicitly allow)
- **Output shape:** `{"decision": string, "constraints": object, "ttl": int}`

### 5.2 Implement read access rules
- **File:** `policies/policy/assistant/v1/read.rego`
- **Rules:**
  - Allow all read actions (`input.action.type == "read"`)
  - Allow read actions for any authenticated actor
  - Set TTL to 300 (5 min cache)

### 5.3 Implement write access rules
- **File:** `policies/policy/assistant/v1/write.rego`
- **Rules:**
  - Low-risk writes in non-prod → `allow` with constraints
  - Medium-risk writes → `require_approval`
  - High-risk writes → `require_approval` with break-glass check
  - Production environment writes → always `require_approval`
  - Account-level blast radius → `require_approval`

### 5.4 Implement environment constraints
- **File:** `policies/policy/assistant/v1/environment.rego`
- **Rules:**
  - Deny writes to environments not in allowed list
  - Maintenance window enforcement (deny outside window for prod)
  - Max target limits per environment tier

### 5.5 Implement actor-based rules
- **File:** `policies/policy/assistant/v1/actors.rego`
- **Rules:**
  - Role-based access: `admin` can break-glass, `operator` can write, `viewer` read-only
  - Tenant isolation: actor's tenant must match context tenant
  - Deny if actor has no roles

### 5.6 Implement break-glass rules
- **File:** `policies/policy/assistant/v1/break_glass.rego`
- **Rules:**
  - Break-glass tier requires `input.resources.break_glass == true`
  - Break-glass actions always require approval regardless of risk
  - Log break-glass usage in decision constraints

### 5.7 Write OPA policy tests
- **File:** `policies/policy/assistant/v1/base_test.rego`
- **Test all decision paths:** allow, deny, require_approval
- **Test all risk levels:** read, low, medium, high
- **Test environment rules:** prod vs non-prod
- **Test actor roles:** admin, operator, viewer, no-role
- **Test break-glass:** with and without header
- **Run with:** `opa test policies/ -v`

### 5.8 Add OPA policy bundle configuration
- **File:** `policies/bundle.yaml` or `policies/.manifest`
- **Define roots and revision**
- **Document how to load policies into OPA:** bundle server, filesystem, or API push

### 5.9 Document policy deployment
- **File:** `policies/README.md`
- **Cover:** How to test policies locally, how to deploy to OPA server, policy input/output contract, example curl commands

---

## 6. Deployment Manifests

### 6.1 Create docker-compose.yml for full stack
- **File:** `deploy/docker-compose.yml`
- **Services:**
  - `postgres` (postgres:15, healthcheck, volume for data)
  - `temporal` (temporalio/auto-setup, depends on postgres)
  - `minio` (object storage, healthcheck)
  - `opa` (openpolicyagent/opa, bundle from policies/)
  - `gateway` (build from docker/gateway.Dockerfile, depends on postgres + temporal + opa)
  - `tool-router` (build from docker/tool-router.Dockerfile, depends on opa)
  - `orchestrator` (build from docker/orchestrator.Dockerfile, depends on postgres + temporal + tool-router)
  - `agent` (build from docker/agent.Dockerfile, depends on gateway)
  - `migrate` (build from docker/gateway.Dockerfile, runs migrations, depends on postgres)
- **Networks:** Single bridge network
- **Volumes:** postgres-data, minio-data

### 6.2 Create Helm chart for Kubernetes deployment
- **Directory:** `deploy/helm/carapulse/`
- **Templates:**
  - `deployment-gateway.yaml` — Deployment + Service + health probes
  - `deployment-tool-router.yaml` — Deployment + Service + Docker socket volume
  - `deployment-orchestrator.yaml` — Deployment + Docker socket volume
  - `deployment-agent.yaml` — Deployment + Service (optional)
  - `job-migrate.yaml` — Job (runs before deployments via helm hook)
  - `configmap.yaml` — Config JSON
  - `secret.yaml` — DSN, API keys, tokens
  - `serviceaccount.yaml` — RBAC for tool-router (kubectl access)
  - `ingress.yaml` — Ingress for gateway
- **values.yaml:** Image tags, replica counts, resource limits, config overrides, connector tokens
- **Chart.yaml:** Name, version, appVersion, dependencies (postgresql, temporal as subcharts or external)

### 6.3 Create Kubernetes RBAC for tool-router
- **File:** `deploy/helm/carapulse/templates/rbac.yaml`
- **ServiceAccount + ClusterRole + ClusterRoleBinding**
- **Permissions:** tool-router needs kubectl access to target cluster (deployments, pods, services, events, nodes)
- **Scope:** Namespace-scoped or cluster-scoped depending on context config

---

## 7. CI/CD Pipeline

### 7.1 Create GitHub Actions workflow for build + test
- **File:** `.github/workflows/ci.yml`
- **Triggers:** Push to any branch, PR to `dev`/`main`
- **Steps:**
  1. Checkout
  2. Setup Go 1.23
  3. `go build ./...`
  4. `go vet ./...`
  5. `go test -race -coverprofile=coverage.out ./...`
  6. Upload coverage artifact

### 7.2 Create GitHub Actions workflow for lint
- **File:** `.github/workflows/lint.yml`
- **Tool:** `golangci-lint` with `.golangci.yml` config
- **Linters to enable:** `errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign`, `gosimple`

### 7.3 Create GitHub Actions workflow for Docker build
- **File:** `.github/workflows/docker.yml`
- **Triggers:** Push to `main`/`dev`, tag push
- **Steps:** Build all 5 images, push to container registry (GHCR or ECR)
- **Tags:** `latest` for main, git SHA for all, semver for tags

### 7.4 Create GitHub Actions workflow for OPA policy tests
- **File:** `.github/workflows/policy.yml`
- **Steps:** Install OPA, run `opa test policies/ -v`

### 7.5 Create golangci-lint configuration
- **File:** `.golangci.yml`
- **Enable:** `errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign`, `gosimple`, `gofmt`, `goimports`
- **Exclude:** Generated files, test files for some linters
- **Timeout:** 5 minutes

### 7.6 Add Makefile
- **File:** `Makefile`
- **Targets:**
  - `build` — `go build ./...`
  - `test` — `go test -race ./...`
  - `lint` — `golangci-lint run`
  - `migrate-up` — `go run ./cmd/migrate -action up`
  - `docker-build` — Build all Docker images
  - `docker-push` — Push images to registry
  - `e2e-up` — `./scripts/e2e-up.sh`
  - `clean` — Remove build artifacts

---

## 8. Structured Logging

### 8.1 Create logger package using slog
- **File:** `internal/logger/logger.go`
- **Use:** Go stdlib `log/slog` (available since Go 1.21, project uses 1.23)
- **Configure:** JSON handler for production, text handler for development
- **Default fields:** `service` (gateway/tool-router/orchestrator/agent), `version`
- **Function:** `New(service string, opts ...Option) *slog.Logger`

### 8.2 Replace log.Printf in gateway
- **File:** `cmd/gateway/main.go`
- **Replace all ~10 `log.Printf` calls** with `slog.Info`, `slog.Error`, `slog.Warn`
- **Add structured fields:** service name, listen address, error details

### 8.3 Replace log.Printf in tool-router
- **File:** `cmd/tool-router/main.go`
- **Replace all ~13 `log.Printf` calls**
- **Add:** tool name, action, execution duration, vault/boundary status

### 8.4 Replace log.Printf in orchestrator
- **File:** `cmd/orchestrator/main.go`
- **Replace all ~12 `log.Printf` calls**
- **Add:** workflow name, activity name, vault/boundary status

### 8.5 Replace log.Printf in agent
- **File:** `cmd/agent/main.go`
- **Replace ~2 calls**

### 8.6 Add request logging middleware to gateway
- **File:** `internal/web/middleware.go` (new or extend existing)
- **Log per request:** method, path, status code, duration_ms, actor_id, trace_id
- **Skip:** /healthz, /readyz (too noisy)

### 8.7 Add request logging middleware to tool-router
- **File:** `cmd/tool-router/main.go` (wrap mux)
- **Log per request:** tool, action, duration_ms, status, actor_id

### 8.8 Add structured logging to audit events
- **File:** `internal/audit/audit.go`
- **Log every audit event insertion with:** event_id, action, decision, actor_id

### 8.9 Add log level configuration
- **Config field:** `log_level` (debug/info/warn/error) in top-level config
- **Default:** `info`
- **File:** `internal/config/config.go` — add `LogLevel string` field

---

## 9. Prometheus Metrics

### 9.1 Add prometheus client dependency
- **Command:** `go get github.com/prometheus/client_golang`

### 9.2 Create metrics package
- **File:** `internal/metrics/metrics.go`
- **Define metrics:**
  - `carapulse_http_request_duration_seconds` (histogram, labels: method, path, status)
  - `carapulse_http_requests_total` (counter, labels: method, path, status)
  - `carapulse_plans_created_total` (counter, labels: environment, risk_level)
  - `carapulse_policy_decisions_total` (counter, labels: decision, action_type, risk_level)
  - `carapulse_tool_executions_total` (counter, labels: tool, action, status)
  - `carapulse_tool_execution_duration_seconds` (histogram, labels: tool, action)
  - `carapulse_approval_requests_total` (counter, labels: status)
  - `carapulse_workflow_completions_total` (counter, labels: workflow, status)
  - `carapulse_audit_events_total` (counter)
  - `carapulse_context_poll_duration_seconds` (histogram, labels: collector)
  - `carapulse_context_poll_errors_total` (counter, labels: collector)

### 9.3 Add /metrics endpoint to gateway
- **File:** `internal/web/http.go`
- **Route:** `GET /metrics` using `promhttp.Handler()`
- **Do NOT wrap in AuthMiddleware**

### 9.4 Add /metrics endpoint to tool-router
- **File:** `cmd/tool-router/main.go`
- **Route:** `GET /metrics`

### 9.5 Instrument gateway HTTP middleware
- **File:** `internal/web/middleware.go`
- **Record:** request duration histogram, request count by status

### 9.6 Instrument tool execution
- **File:** `internal/tools/router.go`
- **Record:** execution duration, success/failure count per tool

### 9.7 Instrument policy decisions
- **File:** `internal/web/policy.go`
- **Record:** decision count by type (allow/deny/require_approval)

### 9.8 Instrument context collectors
- **File:** `internal/context/service.go`
- **Record:** poll duration, error count per collector type

---

## 10. JWKS Caching

### 10.1 Create shared JWKS cache
- **File:** `internal/auth/jwks_cache.go` (new package to deduplicate)
- **Implementation:**
  - `sync.RWMutex`-protected in-memory cache
  - Key: JWKS URL, Value: parsed JWKS + fetch timestamp
  - Configurable TTL (default: 1 hour)
  - Background refresh before TTL expires
  - Exponential backoff on fetch failures (1s → 2s → 4s → ... → 5min max)

### 10.2 Consolidate duplicate JWKS code
- **Current:** `internal/web/jwks.go` and `internal/tools/jwks.go` are near-identical
- **Action:** Delete both, replace with shared `internal/auth/jwks_cache.go`
- **Update imports in:** `internal/web/jwt.go`, `internal/tools/jwt.go`

### 10.3 Add JWKS cache TTL to config
- **Field:** `gateway.jwks_cache_ttl_secs` (default 3600)
- **File:** `internal/config/config.go` — add to `GatewayConfig`

### 10.4 Add cache metrics
- **Metrics:** `carapulse_jwks_cache_hits_total`, `carapulse_jwks_cache_misses_total`, `carapulse_jwks_fetch_errors_total`

---

## 11. Config Validation Expansion

### 11.1 Validate OIDC config completeness
- **File:** `internal/config/config.go`
- **Rule:** If any of `oidc_issuer`, `oidc_client_id`, `oidc_jwks_url` is set, ALL three must be set
- **Apply to:** `GatewayConfig` and `ToolRouterConfig`

### 11.2 Validate LLM config when provider is set
- **Rule:** If `llm.provider` is non-empty, require `llm.model`
- **Rule:** If provider is `openai` or `anthropic`, require `llm.api_key` (unless auth profile is set)

### 11.3 Validate sandbox config consistency
- **Rule:** If `sandbox.enforce == true`, require `sandbox.image`
- **Rule:** If `sandbox.require_seccomp == true`, require `sandbox.seccomp_profile`
- **Rule:** If `sandbox.require_user == true`, require `sandbox.user`

### 11.4 Validate connector config consistency
- **Rule:** If a connector has a `token` set, its `addr` must also be set
- **Apply to:** All connectors (ArgoCD, Grafana, Vault, Boundary, etc.)

### 11.5 Move agent ChatOps validation into Config.Validate()
- **Current:** `cmd/agent/main.go` manually checks `SlackSigningSecret` and `GatewayURL`
- **Move to:** `Config.Validate()` with conditional check: if `chatops.slack_signing_secret` is set, require `gateway_url`

### 11.6 Add config validation tests
- **File:** `internal/config/config_test.go`
- **Test all new validation rules:** valid configs pass, invalid configs return specific errors
- **Test partial OIDC, missing sandbox fields, orphaned tokens, etc.**

### 11.7 Validate storage.workspace_dir exists
- **Rule:** If `storage.workspace_dir` is set, check that directory exists or can be created
- **Note:** Only validate at startup, not in `Validate()`

---

## 12. OpenTelemetry Tracing

### 12.1 Add OpenTelemetry dependencies
- **Command:** `go get go.opentelemetry.io/otel go.opentelemetry.io/otel/sdk go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`

### 12.2 Create tracing initialization
- **File:** `internal/tracing/tracing.go`
- **Setup:** OTLP exporter to Tempo (configurable endpoint)
- **Resource:** service.name, service.version
- **Sampler:** Configurable (always_on for dev, ratio for prod)

### 12.3 Add trace config fields
- **File:** `internal/config/config.go`
- **Fields:** `tracing.enabled`, `tracing.endpoint` (OTLP gRPC), `tracing.sample_rate` (0.0-1.0)

### 12.4 Instrument gateway HTTP handlers
- **File:** `internal/web/middleware.go`
- **Extract or create trace context per request**
- **Span attributes:** method, path, actor_id, tenant_id

### 12.5 Propagate trace context to tool-router
- **File:** `internal/tools/router.go` or `internal/web/http.go` (where tool-router HTTP calls are made)
- **Use:** W3C traceparent header propagation

### 12.6 Instrument tool execution spans
- **File:** `internal/tools/router.go`
- **Span per tool execution** with attributes: tool, action, duration, success

### 12.7 Instrument Temporal workflows
- **File:** `internal/workflows/workflows.go`
- **Use Temporal's OpenTelemetry interceptor** for automatic workflow/activity tracing

### 12.8 Instrument policy evaluation
- **File:** `internal/policy/opa.go`
- **Span per OPA call** with attributes: decision, duration

---

## 13. S3 Go SDK Migration

### 13.1 Replace AWS CLI shelling with Go SDK
- **File:** `internal/storage/object_store.go`
- **Current:** Shells out to `aws s3 cp` and `aws s3 presign`
- **Replace with:** `github.com/aws/aws-sdk-go-v2/service/s3`
- **Methods to rewrite:** `Put()`, `Presign()`
- **Benefits:** No aws CLI dependency, better error handling, connection pooling, no subprocess overhead

### 13.2 Add Go SDK dependencies
- **Command:** `go get github.com/aws/aws-sdk-go-v2 github.com/aws/aws-sdk-go-v2/service/s3 github.com/aws/aws-sdk-go-v2/credentials`

### 13.3 Support MinIO endpoint override
- **Current:** Uses `--endpoint-url` flag with aws CLI
- **SDK equivalent:** Custom endpoint resolver in S3 client config

### 13.4 Update object store tests
- **Add tests using MinIO test container** or mock S3 client

---

## 14. Database Transactions

### 14.1 Add transaction support to DB layer
- **File:** `internal/db/db.go`
- **Add:** `func (d *DB) WithTx(ctx context.Context, fn func(*DB) error) error`
- **Implementation:** `d.raw.BeginTx()`, run fn with tx-scoped DB, commit/rollback
- **Pattern:** The `dbConn` interface already abstracts ExecContext/QueryRowContext — pass tx that implements same interface

### 14.2 Wrap plan creation in transaction
- **File:** `internal/db/queries.go`
- **Current:** `CreatePlan` + `CreateExecution` + approval insert are separate queries
- **Change:** Wrap in single transaction so partial creation can't happen

### 14.3 Wrap execution completion in transaction
- **File:** `internal/db/executions.go`
- **Current:** `CompleteExecution` updates execution + may update multiple tool calls
- **Change:** Atomic update of execution status + all tool call statuses

### 14.4 Wrap context snapshot ingestion in transaction
- **File:** `internal/db/context.go`
- **Current:** `UpsertContextNode` and `UpsertContextEdge` called in loop
- **Change:** Batch upserts within single transaction for consistency

---

## 15. Rate Limiting

### 15.1 Add rate limiter middleware to gateway
- **File:** `internal/web/ratelimit.go` (new)
- **Implementation:** Token bucket per actor ID (from JWT claims)
- **Config fields:** `gateway.rate_limit_rps` (default 10), `gateway.rate_limit_burst` (default 20)
- **Library:** `golang.org/x/time/rate`
- **Response:** 429 Too Many Requests with `Retry-After` header

### 15.2 Add rate limiter to tool-router
- **File:** `cmd/tool-router/main.go`
- **Rate limit per tool execution** to prevent abuse
- **Config:** `tool_router.rate_limit_rps`

---

## 16. Token Refresh

### 16.1 Implement LLM token refresh
- **File:** `internal/llm/auth.go`
- **Current:** `profile.Expired()` checks but no refresh mechanism
- **Add:** Background goroutine that refreshes tokens before expiry
- **For Codex:** Re-read `OPENAI_ACCESS_TOKEN` or re-authenticate

### 16.2 Implement OIDC token refresh for connectors
- **File:** `internal/web/auth.go` or new `internal/auth/refresh.go`
- **For connectors with OIDC:** Refresh token flow using OIDC refresh_token grant
- **Cache refreshed tokens** with TTL

---

## 17. Testing Improvements

### 17.1 Add integration test for migration runner
- **File:** `cmd/migrate/main_test.go`
- **Use:** Testcontainers for PostgreSQL
- **Test:** Run all migrations up, verify schema, run down, verify clean

### 17.2 Add integration test for health endpoints
- **File:** `internal/web/health_test.go`
- **Test:** /healthz returns 200, /readyz returns 503 when DB is down, 200 when healthy

### 17.3 Add integration test for graceful shutdown
- **Test:** Start service, send SIGTERM, verify goroutines exit within timeout, verify connections close

### 17.4 Add end-to-end Docker Compose test
- **File:** `.github/workflows/e2e.yml` or `scripts/e2e-docker-test.sh`
- **Spin up full stack via docker-compose**, run migrations, hit health endpoints, create a plan, verify audit trail

---

## 18. Documentation

### 18.1 Create deployment guide
- **File:** `docs/deployment.md`
- **Cover:** Docker Compose quickstart, Kubernetes/Helm deployment, config reference, migration procedure, health check URLs

### 18.2 Create operations guide
- **File:** `docs/operations.md`
- **Cover:** Log format, metric names, alert recommendations, troubleshooting common issues, break-glass procedure

### 18.3 Create policy authoring guide
- **File:** `policies/README.md`
- **Cover:** OPA input/output contract, example rules, testing with `opa test`, deployment to OPA server

### 18.4 Update CLAUDE.md with new commands
- **File:** `CLAUDE.md`
- **Add:** `go run ./cmd/migrate`, `make` targets, Docker build commands, health endpoint URLs

### 18.5 Create architecture diagram
- **File:** `docs/architecture.md`
- **ASCII or Mermaid diagram** showing: gateway ↔ tool-router, gateway ↔ orchestrator ↔ Temporal, gateway ↔ OPA, all ↔ Postgres

---

## Task Dependency Graph

```
Dockerfiles (1.*) ←── depends on ──→ Migration Runner (2.*)
                  ←── depends on ──→ Health Endpoints (3.*)
                  ←── depends on ──→ Graceful Shutdown (4.*)

Docker Compose (6.1) ←── depends on ──→ Dockerfiles (1.*)
                      ←── depends on ──→ OPA Rules (5.*)
                      ←── depends on ──→ Migration Runner (2.*)

Helm Chart (6.2) ←── depends on ──→ Dockerfiles (1.*)
                 ←── depends on ──→ Health Endpoints (3.*)
                 ←── depends on ──→ Graceful Shutdown (4.*)

CI/CD (7.*) ←── depends on ──→ Dockerfiles (1.*)
            ←── depends on ──→ Lint Config (7.5)

Metrics (9.*) ←── depends on ──→ Structured Logging (8.*)
Tracing (12.*) ←── depends on ──→ Structured Logging (8.*)

Everything else is independent.
```

## Recommended Implementation Order

**Phase 1 — Deployable (do first):**
1. Migration Runner (2.1-2.5)
2. Health Endpoints (3.1-3.4)
3. Graceful Shutdown (4.1-4.4)
4. Dockerfiles (1.1-1.5, 1.7)
5. Docker Compose (6.1)

**Phase 2 — Secure & Observable:**
6. OPA Policy Rules (5.1-5.7)
7. Config Validation (11.1-11.6)
8. JWKS Caching (10.1-10.3)
9. Structured Logging (8.1-8.9)

**Phase 3 — Production-Grade:**
10. CI/CD Pipeline (7.1-7.6)
11. Prometheus Metrics (9.1-9.8)
12. DB Transactions (14.1-14.4)
13. Makefile (7.6)
14. Helm Chart (6.2-6.3)

**Phase 4 — Polish:**
15. OpenTelemetry Tracing (12.1-12.8)
16. S3 Go SDK (13.1-13.4)
17. Rate Limiting (15.1-15.2)
18. Token Refresh (16.1-16.2)
19. Documentation (18.1-18.5)
20. Testing (17.1-17.4)
