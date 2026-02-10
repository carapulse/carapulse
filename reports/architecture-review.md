# Carapulse Architecture Review

**Date:** 2026-02-08
**Reviewer:** Technical Architect Agent
**Scope:** System design, scalability, resilience, security, and production readiness

---

## Executive Summary

Carapulse is a well-structured, single-tenant DevOps/SRE assistant with clean service boundaries, comprehensive policy enforcement, and solid security fundamentals. The architecture follows sound principles (CLI-first execution, approval gates, ContextRef scoping, OPA policy integration). However, several areas need attention before production deployment at scale: notably the absence of database transactions for multi-step operations, limited retry logic outside Temporal, no rate limiting on API endpoints, and unbounded in-memory state in the event/log hubs.

**Overall Production Readiness: 7/10** -- Solid foundation with specific gaps to address.

---

## 1. Service Boundaries

**Rating: GOOD -- Needs minor work**

### Architecture

```
                    +------------------+
                    |    Clients       |
                    | (API / Slack /   |
                    |  Webhooks / UI)  |
                    +--------+---------+
                             |
                    +--------v---------+
                    |    Gateway        |
                    |    (:8080)        |
                    |  - REST API       |
                    |  - SSE/WS         |
                    |  - Event Loop     |
                    |  - Scheduler      |
                    |  - Approval Watch |
                    +---+----------+---+
                        |          |
              +---------v--+  +----v-----------+
              | Tool Router |  |  Orchestrator  |
              |  (:8081)    |  | (Temporal Wkr) |
              | - CLI exec  |  | - Workflows    |
              | - API exec  |  | - Activities   |
              | - Sandbox   |  | - Rollbacks    |
              | - Redaction  |  +-------+--------+
              | - Policy     |          |
              +------+------+   +------v------+
                     |          |   Temporal   |
              +------v------+  +------+-------+
              | CLI Tools   |         |
              | aws/kubectl |  +------v------+
              | helm/vault  |  |  Postgres   |
              | argocd/git  |  +-------------+
              +-----------+
```

### Separation of Concerns

**Strengths:**
- Gateway, Tool Router, and Orchestrator have clearly distinct responsibilities
- Gateway owns HTTP API, event streaming, scheduling, approval watching
- Tool Router owns tool execution, sandboxing, redaction, policy enforcement
- Orchestrator owns durable workflow execution via Temporal
- Clean interface boundaries: `ExecutionStarter`, `DBWriter`, `ApprovalCreator`

**Concerns:**

1. **Gateway is a "god service"**: The gateway runs 5+ background goroutines (alert poller, context service, approval watcher, scheduler, event loop). It has accumulated significant responsibility. While not a blocking issue for single-tenant deployment, it makes the gateway harder to reason about and test in isolation.

2. **Duplicated sandbox/router initialization**: Both `cmd/tool-router/main.go` and `cmd/orchestrator/main.go` contain nearly identical sandbox configuration (~20 lines of field assignment). This is a code smell suggesting a shared constructor or builder is needed.

3. **Duplicated Vault/Boundary initialization**: Both tool-router and orchestrator have identical blocks for Vault agent startup, health checks, token renewal, and Boundary session management. This is ~100 lines of duplicated infrastructure setup.

4. **Import coupling**: `internal/workflows/starter.go` imports `internal/web` for `ContextRef` and `PlanStep` types. This creates a dependency from the orchestrator layer back to the web layer. These shared types should live in a neutral package (e.g., `internal/model` or `internal/types`).

### Import Graph (Simplified)

```
cmd/gateway --> internal/web --> internal/db
                             --> internal/policy
                             --> internal/llm
                             --> internal/context
                             --> internal/tools (RouterClient)
                             --> internal/workflows (TemporalStarter)

cmd/tool-router --> internal/tools (Router, Sandbox, Server)
                --> internal/policy
                --> internal/secrets

cmd/orchestrator --> internal/workflows --> internal/tools
                 --> internal/db
                 --> internal/secrets
                 --> internal/web (ContextRef, PlanStep via starter.go)  <-- CONCERN
```

---

## 2. Data Flow Analysis

**Rating: GOOD -- Needs minor work**

### Critical Path: Plan Creation -> Approval -> Execution

```
1. POST /v1/plans
   |
   +-> AuthMiddleware (JWT validation + JWKS cache)
   +-> Decode request body
   +-> riskFromIntent() -- static risk classification
   +-> validateContextRefStrict() -- input validation
   +-> policyDecision() -- OPA policy check
   +-> mergeConstraints() -- policy + request constraints
   +-> Diagnostics.Collect() -- gather infra evidence (optional)
   +-> Planner.Plan() -- LLM generates plan text (optional)
   +-> parsePlanSteps() -- extract structured steps
   +-> DB.CreatePlan() -- INSERT into plans table
   +-> DB.CreateApproval() -- INSERT into approvals table
   +-> Approvals.CreateApprovalIssue() -- Create Linear issue (optional)
   +-> emit("plan.updated") -- SSE/WS notification
   |
   Response: { plan_id, plan, created_at }

2. Approval Flow
   |
   +-> Linear issue created with plan details
   +-> ApprovalWatcher polls Linear API (configurable interval)
   +-> When issue resolved: DB.UpdateApprovalStatusByPlan("approved")
   |
   OR
   |
   +-> POST /v1/approvals { plan_id, status: "approved" }
   +-> AuthMiddleware + policyCheck
   +-> DB.UpdateApprovalStatusByPlan()

3. POST /v1/plans/{id}:execute
   |
   +-> AuthMiddleware
   +-> DB.GetPlan() -- load plan
   +-> requireSession() -- session check (optional)
   +-> enforceSessionMatch() -- plan-session binding
   +-> policyDecision() -- OPA check for plan.execute
   +-> enforceConstraints() -- time windows, target limits
   +-> approvalStatus() -- verify approval exists + approved
   +-> DB.CreateExecution() -- INSERT into executions
   +-> Executor.StartExecution() -- dispatch to Temporal
   |   |
   |   +-> Temporal: PlanExecutionWorkflow
   |       +-> UpdateExecutionStatus("running")
   |       +-> For each act step:
   |       |   +-> ExecuteStep activity
   |       |       +-> Runtime.Router.Execute()
   |       |           +-> CLI-first, API fallback
   |       |           +-> Sandbox enforcement
   |       |           +-> Redaction
   |       |       +-> Store input/output to object store
   |       |       +-> Record evidence
   |       +-> For each verify step:
   |       |   +-> ExecuteStep activity
   |       |   +-> On failure: rollback act steps in reverse
   |       +-> CompleteExecution("succeeded" | "failed" | "rolled_back")
   |
   OR (without Temporal)
   |
   +-> Executor.Run() polls for pending executions
       +-> executePlan() -- same step/verify/rollback logic
```

### Bottlenecks and SPOFs

| Component | Role | SPOF? | Notes |
|-----------|------|-------|-------|
| Postgres | Persistent state for all entities | YES | Single DB, no replicas configured |
| Temporal | Workflow orchestration | NO* | Gateway works without it; executions fail gracefully |
| OPA | Policy evaluation | Partial | `FailOpenReads` config allows reads to continue |
| LLM Provider | Plan generation | NO | Plan creation works without LLM (empty plan text) |
| Linear API | Approval management | NO | Can use API-based approvals instead |
| Object Store | Input/output artifacts | NO | Executor continues without it (refs empty) |

*Temporal is a SPOF for durable workflow execution specifically. The direct `Executor` path exists as an alternative.

---

## 3. Scalability Assessment

**Rating: NEEDS WORK**

### Horizontal Scaling Capability

| Service | Horizontally Scalable? | Constraints |
|---------|----------------------|-------------|
| Gateway | Partially | EventHub/LogHub are in-memory; multiple instances lose event pub/sub coherence. Scheduler and approval watcher would need distributed locking. |
| Tool Router | YES | Stateless request handler. Multiple instances work behind a load balancer. |
| Orchestrator | YES | Temporal handles worker distribution natively. Multiple workers compete for tasks. |

### Key Bottlenecks

1. **No DB connection pooling configuration**: `db.NewDB()` calls `sql.Open("postgres", dsn)` with no `SetMaxOpenConns`, `SetMaxIdleConns`, or `SetConnMaxLifetime`. Go's `database/sql` defaults to unlimited connections, which can overwhelm Postgres under load.

2. **In-memory EventHub/LogHub**: Both hubs store state in memory with no persistence or cross-instance coordination. If the gateway restarts, all SSE/WS subscribers disconnect and log history is lost. If multiple gateway instances run, events are not shared.

3. **LogHub unbounded history growth**: The LogHub limits to 200 lines per execution ID, but never cleans up completed execution entries. Over time, `h.history` grows without bound as executions accumulate.

4. **EventHub channel backpressure**: Events are published with `select { case ch <- ev: default: }` -- dropping events when subscriber channels are full (buffer=8). This is correct for resilience but means clients can miss events silently.

5. **Single-tenant model**: The entire system is designed for single-tenant deployment. Multi-tenancy would require ContextRef-based data isolation at the DB layer (not present), per-tenant connection pools, and tenant-aware scheduling.

6. **Executor sequential processing**: The direct `Executor` processes plans sequentially within each batch. A long-running tool execution blocks all other pending executions. The Temporal path handles this better via parallel workers.

7. **No request body size limits**: HTTP handlers use `json.NewDecoder(r.Body).Decode(&req)` without `http.MaxBytesReader`. A malicious client can send arbitrarily large payloads.

### Concurrency Controls

- Tool Router: No concurrency limit on parallel tool executions
- Executor: `MaxBatch=10` per poll cycle, but executes sequentially
- Temporal: Uses `worker.Options{}` with default concurrency (configurable)
- Context pollers: Run on fixed intervals; no parallel poll limiting

---

## 4. Resilience Patterns

**Rating: NEEDS WORK**

### Retry Logic

| Component | Retry? | Implementation |
|-----------|--------|---------------|
| Temporal workflows | YES | `RetryPolicy{InitialInterval: 1s, Backoff: 2.0, MaxInterval: 1m, MaxAttempts: 5}` |
| JWKS cache | YES | Exponential backoff (1s -> 2s -> ... -> 5min cap) |
| Vault agent | YES | Configurable `retry_max_backoff` |
| Tool Router CLI execution | NO | Single attempt; failure is immediate |
| Direct Executor | NO | Single attempt per step; failure triggers rollback |
| OPA policy evaluation | NO | Single attempt; failure blocks the request |
| LLM provider calls | NO | Single attempt; failure returns 502 |
| DB operations | NO | Single attempt; failure returns 500 |
| Linear API (approvals) | YES | Approval watcher polls on interval |
| Alert poller | Implicit | Polls on interval; errors are logged and next poll continues |

### Circuit Breakers

**None found.** There are no circuit breaker implementations in the codebase. This means:
- A failing OPA service will cause every write request to fail
- A slow Postgres will block all request handlers
- A hanging CLI tool will block the executor (unless the context times out)

### Timeout Analysis

| Component | Timeout | Notes |
|-----------|---------|-------|
| OPA policy HTTP client | 5 seconds | `http.Client{Timeout: 5 * time.Second}` |
| JWKS fetch | 10 seconds | Via `context.WithTimeout` |
| LLM HTTP client | 5 seconds (default) | Configurable via `llm.timeout_ms` |
| Health check DB ping | 2 seconds | `context.WithTimeout(r.Context(), 2*time.Second)` |
| Health check Temporal | 2 seconds | Same |
| Sandbox runtime check | 2 seconds | In tool-router readyz |
| HTTP server shutdown | 30 seconds | `context.WithTimeout` + force exit timer |
| Temporal activity | 10 minutes | `StartToCloseTimeout: 10 * time.Minute` |
| Tool CLI execution | **NONE** | No timeout on `exec.CommandContext` beyond the parent context |
| Direct executor steps | **NONE** | No per-step timeout |

### What Happens When...

**Temporal is down:**
- Gateway startup: Temporal client dial fails -> gateway exits with error
- This is problematic: the gateway should be able to start without Temporal (degraded mode). Config validation requires `orchestrator.temporal_addr` even when gateway doesn't strictly need it.

**Postgres is slow:**
- All HTTP handlers that touch DB will block/timeout based on the client's HTTP timeout
- No statement-level timeouts configured
- No connection pool limits -> potential connection exhaustion

**A CLI tool hangs:**
- Sandbox.Run() uses `exec.CommandContext(ctx, ...)` which respects context cancellation
- But the parent context for tool execution has no explicit timeout
- In the Temporal path, the activity has a 10-minute timeout
- In the direct Executor path, there is no timeout beyond the parent context

**OPA is down:**
- Write requests: Fail with 403 (policy denied)
- Read requests: Configurable via `fail_open_reads` -- can allow reads without policy check
- Tool executions: Tool router requires OPA (`policy required` error) unless checker is nil

---

## 5. Security Architecture

**Rating: GOOD -- Production-ready with caveats**

### Authentication Flow

```
Request --> ParseBearer(r)
        --> ParseJWTClaims(token)     -- decode payload (no verification)
        --> validateClaims(claims)    -- issuer, audience, expiry, nbf
        --> VerifyJWTSignature(token) -- RS256 via JWKS endpoint
        --> WithActor(ctx, actor)     -- inject Actor into context
```

**Strengths:**
- Full JWT validation: issuer, audience, expiry, nbf checks
- RS256 signature verification against JWKS endpoint
- JWKS caching with configurable TTL and exponential backoff on fetch failures
- All API routes wrapped in `AuthMiddleware`
- Actor identity (sub, email, groups) propagated through context

**Concerns:**

1. **Only RS256 supported**: `VerifyJWTSignature` rejects any algorithm other than RS256. This is actually good for security (prevents algorithm confusion attacks), but should be documented.

2. **Token-based auth for tool-router**: The tool-router accepts a static bearer token (`auth_token`) as an alternative to JWT. This is convenient for service-to-service auth but the token is stored in plaintext in the JSON config.

3. **No token revocation**: There's no mechanism to revoke or blacklist compromised JWTs before expiry. The JWKS cache means key rotation takes up to `jwks_cache_ttl_secs` to propagate.

4. **Config contains secrets in plaintext**: The JSON config file holds `api_key`, `auth_token`, `slack_bot_token`, `slack_signing_secret`, `approle_secret`, and various connector tokens. In the Helm chart, this goes into a ConfigMap (not a Secret).

### Sandbox Isolation

**Strengths:**
- Container-based sandboxing with Docker/OCI runtime
- `--network=none` when no egress allowlist configured
- Read-only root filesystem support
- tmpfs mount points
- Seccomp profile enforcement
- `no-new-privileges` security option
- Capability dropping (`--cap-drop`)
- Non-root user enforcement
- Egress proxy with allowlist filtering
- `RequireOnWrite` mode: sandbox mandatory for write operations
- Multiple enforcement booleans for defense-in-depth

**Concerns:**
1. **Enforce mode requires all security features**: When `Enforce=true` and `Enabled=true`, the sandbox requires seccomp, no-new-privs, user, and drop-caps all to be set. This is strict but could block legitimate deployments that don't have seccomp profiles.

### Policy Enforcement (OPA)

```
Handler --> policyDecision(action, type, context, risk, targets)
        --> PolicyService.Evaluate(PolicyInput{actor, action, context, resources, risk, time})
        --> OPA HTTP API POST /v1/data/<package>
        --> PolicyDecision{decision, constraints, ttl}
```

**Strengths:**
- Centralized policy evaluation via OPA
- Rich policy input (actor, action, context, risk level, blast radius, time)
- Policy constraints merged with request constraints
- Break-glass mechanism via `X-Break-Glass` header
- Separate policy enforcement at both gateway and tool-router levels
- 7 Rego rule files in `policies/`

**Concerns:**
1. **No policy decision caching**: Every request makes a fresh OPA call. The `PolicyDecision` includes a `TTL` field, but it is never used for caching.
2. **Evaluator nil-checker defaults to allow**: When `Evaluator.Checker` is nil, `Check()` returns `{Decision: "allow"}`. This means policy enforcement is silently skipped if OPA is not configured.

### Secret Management

- Vault agent lifecycle management (auto-auth, template rendering, token sink)
- Boundary session/tunnel management for database access
- Secret redaction in tool output via configurable regex patterns
- Redaction applied to both CLI output and log lines

---

## 6. Config Management

**Rating: NEEDS WORK**

### Single JSON Config Approach

**Strengths:**
- Single config struct shared by all services -- simple mental model
- Validation at load time (~25 checks)
- Each service reads only the sections it needs

**Concerns:**

1. **Secrets in plaintext JSON**: API keys, tokens, and secrets are stored alongside non-sensitive config. The Helm chart mounts this as a ConfigMap. Should use Kubernetes Secrets, external secret management (External Secrets Operator, Vault CSI), or environment variable injection for sensitive values.

2. **Config validation rejects partial configs**: `Config.Validate()` requires `gateway.http_addr`, `policy.opa_url`, `orchestrator.temporal_addr`, and `storage.postgres_dsn` all to be present. This means you cannot run just the gateway without a Temporal address in the config, even though the code handles nil Temporal clients.

3. **No config hot-reload**: Changing configuration requires a full restart of all services. For a single-tenant system this is acceptable, but config changes to things like poll intervals, feature flags, or connector addresses should ideally not require downtime.

4. **No environment variable override**: Configuration can only be loaded from a JSON file. There is no support for environment variable overrides (e.g., `CARAPULSE_STORAGE_POSTGRES_DSN`), which is a Kubernetes deployment best practice for secrets.

---

## 7. Observability

**Rating: GOOD -- Needs minor work**

### Structured Logging

- Uses `log/slog` with JSON (default) or text format
- Service name tagged on all log lines
- Log level configurable via `LOG_LEVEL` env var
- stdlib `log` redirected to slog for consistent output
- Tool execution output redacted before logging

### Prometheus Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `carapulse_http_requests_total` | Counter | method, path, status |
| `carapulse_http_request_duration_seconds` | Histogram | method, path |
| `carapulse_tool_executions_total` | Counter | tool, action, outcome |
| `carapulse_tool_execution_duration_seconds` | Histogram | tool, action |
| `carapulse_workflow_executions_total` | Counter | workflow, outcome |
| `carapulse_policy_decisions_total` | Counter | decision |
| `carapulse_approvals_total` | Counter | status |
| `carapulse_active_websocket_connections` | Gauge | -- |
| `carapulse_active_sse_connections` | Gauge | -- |

**Strengths:**
- Path normalization to prevent high cardinality (`/v1/plans/abc123` -> `/v1/plans`)
- HTTP middleware for automatic instrumentation
- All three services expose `/metrics`
- Good breadth: HTTP, tools, workflows, policy, approvals, connections

**Concerns:**

1. **Tool metrics not wired in router_exec.go**: The `ToolExecutionsTotal` and `ToolExecutionDuration` metrics are declared but I see no evidence of them being incremented in `Router.Execute()`. They need to be instrumented at the execution point.

2. **No tracing**: There is no distributed tracing (OpenTelemetry, Jaeger). The `TempoPoller` collects traces from external services but Carapulse itself does not emit traces. For debugging cross-service request flows (gateway -> tool-router -> CLI -> result), this is a significant gap.

3. **No alerting rules**: The metrics are exposed but no Prometheus alerting rules or Grafana dashboards are included. Key alerts needed: error rate spikes, high latency P99, execution failures, approval timeouts, context collector failures.

4. **Health checks solid**: Both `/healthz` (liveness) and `/readyz` (readiness) are implemented with appropriate dependency checks (DB ping, Temporal health, goroutine status).

---

## 8. API Design

**Rating: GOOD -- Needs minor work**

### REST Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/v1/plans` | Create a plan |
| GET | `/v1/plans/{id}` | Get plan details |
| GET | `/v1/plans/{id}/diff` | Get plan diff/changes |
| GET | `/v1/plans/{id}/risk` | Get risk assessment |
| POST | `/v1/plans/{id}:execute` | Execute a plan |
| POST | `/v1/approvals` | Create/update approval |
| GET | `/v1/executions/{id}` | Get execution status |
| GET | `/v1/executions/{id}/logs` | Stream execution logs (SSE) |
| GET | `/v1/audit/events` | List audit events (with filters) |
| GET | `/v1/context/services` | List context services |
| GET | `/v1/context/snapshots` | List context snapshots |
| GET | `/v1/context/snapshots/{id}` | Get snapshot details |
| POST | `/v1/context/refresh` | Trigger context refresh |
| GET | `/v1/context/graph` | Get service graph |
| POST | `/v1/schedules` | Create a schedule |
| GET | `/v1/schedules` | List schedules |
| CRUD | `/v1/playbooks[/{id}]` | Manage playbooks |
| CRUD | `/v1/runbooks[/{id}]` | Manage runbooks |
| CRUD | `/v1/sessions[/{id}]` | Manage sessions |
| CRUD | `/v1/workflows[/{id}]` | Manage workflow catalog |
| CRUD | `/v1/memory[/{id}]` | Manage operator memory |
| POST | `/v1/hooks/{source}` | Receive webhooks |
| GET | `/v1/ws` | WebSocket event stream |

**Strengths:**
- Consistent `/v1/` versioning prefix
- Google-style custom methods (`:execute`) instead of non-RESTful verbs
- Content-Type consistently set to `application/json`
- Proper HTTP status codes (400, 401, 403, 404, 405, 500, 502, 503)
- All routes behind `AuthMiddleware`
- SSE for log streaming, WebSocket for events

**Concerns:**

1. **No pagination**: `ListSchedules`, `ListPlaybooks`, `ListRunbooks`, `ListSessions`, `ListWorkflowCatalog` all return full result sets. The DB queries use `jsonb_agg` which loads everything into memory. This will not scale.

2. **No request validation framework**: Input validation is done ad-hoc in each handler. There's no consistent error response format -- sometimes plain text (`"invalid json"`), sometimes structured JSON.

3. **Missing LIST for plans/executions**: There is no `GET /v1/plans` (list all plans) or `GET /v1/executions` (list all executions) endpoint. These are fundamental for any dashboard or management UI.

4. **Error responses inconsistent**: Some handlers return `http.Error(w, "message", code)` (plain text), while others return JSON. A standard error envelope (`{"error": "code", "message": "detail"}`) would be more API-friendly.

5. **No rate limiting**: No rate limiting middleware on any endpoint. The webhook endpoints (`/v1/hooks/*`) are particularly vulnerable -- an attacker could trigger unlimited plan creation.

6. **No CORS headers**: The gateway does not set CORS headers, which will block browser-based API clients or the web UI if served from a different origin.

---

## 9. Summary: Production Readiness Ratings

| Area | Rating | Key Issues |
|------|--------|------------|
| Service Boundaries | GOOD | Import coupling via starter.go; duplicated init code |
| Data Flow | GOOD | Clean critical path; proper approval gates |
| Scalability | NEEDS WORK | No DB pool config; in-memory event hubs; no pagination |
| Resilience | NEEDS WORK | No circuit breakers; missing timeouts on tool execution; no retry outside Temporal |
| Security | GOOD | Strong JWT/OIDC; comprehensive sandbox; good policy enforcement |
| Config | NEEDS WORK | Plaintext secrets; no env var overrides; validation too strict |
| Observability | GOOD | Good metrics/logging; missing distributed tracing |
| API Design | GOOD | Clean REST design; missing pagination and rate limiting |

---

## 10. Priority Recommendations

### P0 -- Must fix before production

1. **Add DB connection pool limits**: Set `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime` on `*sql.DB`. Without this, the system can exhaust Postgres connections under load.

2. **Add request body size limits**: Wrap request bodies with `http.MaxBytesReader(w, r.Body, maxBytes)` to prevent OOM from oversized payloads.

3. **Move secrets out of ConfigMap**: Use Kubernetes Secrets, External Secrets Operator, or environment variables for sensitive config values.

4. **Add per-step timeout for direct Executor**: Without this, a hanging CLI tool blocks all execution progress indefinitely.

### P1 -- Should fix before production

5. **Add rate limiting middleware**: At minimum on webhook endpoints and plan creation. Use a token bucket or sliding window rate limiter.

6. **Add pagination to list endpoints**: Cursor-based pagination for all list queries to prevent unbounded result sets.

7. **Add LogHub/EventHub memory cleanup**: Implement TTL-based eviction for completed execution log history. Consider an LRU eviction policy.

8. **Wire tool execution metrics**: Instrument `ToolExecutionsTotal` and `ToolExecutionDuration` in `Router.Execute()`.

9. **Relax config validation**: Allow services to start with partial config. Gateway should not require `orchestrator.temporal_addr` to function.

### P2 -- Should fix for operational maturity

10. **Add distributed tracing**: Integrate OpenTelemetry to trace requests across gateway -> tool-router -> CLI execution.

11. **Add circuit breakers**: For OPA and LLM provider calls. Use a library like `sony/gobreaker` or implement a simple state machine.

12. **Extract shared types**: Move `ContextRef`, `PlanStep`, and related types from `internal/web` to `internal/model` to break the import cycle in `starter.go`.

13. **Add DB transactions**: Multi-step DB operations (plan creation + step insertion + approval creation) should be wrapped in transactions.

14. **Add standard error response envelope**: Consistent JSON error format across all endpoints.

15. **Add CORS middleware**: Configurable CORS headers for browser-based API access.

---

## Appendix: File References

| Area | Key Files |
|------|-----------|
| Gateway | `cmd/gateway/main.go` |
| Tool Router | `cmd/tool-router/main.go` |
| Orchestrator | `cmd/orchestrator/main.go` |
| HTTP handlers | `internal/web/http.go` |
| Auth/JWT | `internal/web/auth.go`, `internal/web/jwt.go` |
| JWKS cache | `internal/auth/jwks_cache.go` |
| Tool execution | `internal/tools/router_exec.go` |
| Sandbox | `internal/tools/sandbox.go` |
| Redaction | `internal/tools/redact.go` |
| Tool registry | `internal/tools/registry.go` |
| Policy (OPA) | `internal/policy/opa.go`, `internal/policy/middleware.go` |
| DB layer | `internal/db/db.go`, `internal/db/queries.go` |
| Workflows | `internal/workflows/workflows.go`, `internal/workflows/executor.go` |
| Temporal wrappers | `internal/workflows/temporal_workflow.go` |
| Events/Logging | `internal/web/events.go`, `internal/logging/logging.go` |
| Metrics | `internal/metrics/metrics.go` |
| Config | `internal/config/config.go` |
| Secrets | `internal/secrets/vault_agent.go`, `internal/secrets/boundary.go` |
