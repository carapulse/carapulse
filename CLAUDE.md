# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Carapulse

Carapulse is an open-source, autonomous DevOps/SRE assistant. Single-tenant, self-hosted. It plans, executes, and verifies infrastructure operations with approval gates, policy enforcement, and audit trails. Stack: Go + Temporal + Postgres + OPA.

## Build & Test

```bash
go build ./...                          # Build everything
go test ./...                           # Run all tests
go test ./internal/tools/...            # Run tests for one package
go test ./internal/web/... -run TestHandlePlans  # Run a single test
go test ./... -count=1                  # Skip test cache
go test -race ./...                     # Race detector
go test -coverprofile=coverage.out ./...  # Coverage
```

Linting via `golangci-lint` (config in `.golangci.yml`). No Makefile. Tests use stdlib `testing` only (no testify assertions beyond what's in go.sum as indirect dep). No build tags.

## Service Architecture

Six binaries in `cmd/`, three core services + three utilities:

**Core services** (all take `-config <path>` for a JSON config file):

- **gateway** (`:8080`) — HTTP API server. Registers all `/v1/*` routes (plans, approvals, executions, sessions, schedules, playbooks, runbooks, workflows, hooks, context, memory). Hosts SSE/WebSocket event streams and an optional web UI. Starts the alert poller, context service, approval watcher, and scheduler as background goroutines.
- **tool-router** (`:8081`) — Tool execution proxy. Receives `ExecuteRequest{Tool, Action, Input}`, routes to CLI-first execution (falls back to API if CLI not found), applies sandbox enforcement, egress allowlists, secret redaction, and OPA policy checks.
- **orchestrator** — Temporal worker. Registers durable workflows (`GitOpsDeployWorkflow`, `HelmReleaseWorkflow`, `ScaleServiceWorkflow`, `IncidentRemediationWorkflow`, `SecretRotationWorkflow`) and their activities. Connects to Vault/Boundary for secret management.

**Utilities:**

- **agent** — Slack ChatOps bridge. Receives Slack commands and forwards them to the gateway.
- **assistantctl** — CLI client. Subcommands: `plan create/approve`, `exec logs`, `context refresh`, `policy test`, `llm`, `schedule create/list`, `workflow replay`.
- **sandbox-exec** — Standalone sandboxed command runner.

## Key Internal Packages

- `internal/config` — Single `Config` struct loaded from JSON. All services share the same config schema; each reads the sections it needs. Validates ~25+ fields at load time.
- `internal/tools` — Tool registry, router, sandbox, schema validation, redaction, HTTP client builder. Tool definitions live in `tools/schemas/*.json` (one per tool: aws, vault, kubectl, helm, argocd, etc.). The Router enforces CLI-first execution.
- `internal/web` — HTTP handlers, event loop, SSE/WS hubs, plan parsing/helpers, risk scoring, policy middleware, scheduler, workflow catalog. `Server` is the central struct wiring DB, policy, approvals, context, diagnostics, and LLM planner.
- `internal/workflows` — Durable workflow definitions (both pure Go and Temporal-wrapped). `Runtime` wraps Router+Sandbox+HTTPClients. `Executor` polls for pending executions and runs plan steps. Activities are individual tool operations (ArgoSync, HelmUpgrade, KubectlScale, etc.).
- `internal/db` — Postgres via `database/sql` with raw SQL queries (no ORM). Migrations in `migrations/` numbered sequentially (`0001_init.sql` through `0020_*`). Applied via `cmd/migrate` (goose-based).
- `internal/llm` — LLM router supporting OpenAI, Anthropic, and Codex providers. Auth via OIDC/OAuth or static keys. Prompt redaction before sending to LLM.
- `internal/policy` — OPA/Rego integration. Sends `PolicyInput{Actor, Action, Context, Resources, Risk}` to OPA and gets back `PolicyDecision{Decision, Constraints, TTL}`. Package: `policy.assistant.v1`. Rules in `policies/*.rego`.
- `internal/context` — Runtime context service. Pollers and watchers collect infrastructure state from K8s, Helm, ArgoCD, Prometheus, Thanos, Alertmanager, Grafana, Tempo, AWS. Produces service graph snapshots.
- `internal/secrets` — Vault agent lifecycle, Boundary session/tunnel management, secret redaction.
- `internal/approvals` — Linear issue-based approval workflow with polling watcher.
- `internal/logging` — Structured logging via `log/slog`. JSON (default) or text format. Configurable via `LOG_FORMAT` and `LOG_LEVEL` env vars.
- `internal/metrics` — Prometheus metrics (request counts, durations, tool executions, workflow runs, policy decisions, approvals, active connections). HTTP middleware for automatic instrumentation. All services expose `/metrics`.

## Design Patterns

- **Dependency injection via package-level vars**: Services use `var newDB = db.NewDB`, `var listen = http.ListenAndServe`, etc. Tests override these vars to inject fakes. This is the primary testing strategy — no interface mocking frameworks.
- **CLI-first tool execution**: The tool router tries CLI tools first (aws, kubectl, helm, vault, argocd, git, gh, glab), falling back to HTTP API only when CLI is unavailable. See `tools/registry.go` for the full tool list.
- **Approval gates**: Every write workflow calls `RequireApproval()` before mutating operations. Approvals flow through Linear issues or the gateway API.
- **ContextRef everywhere**: A `ContextRef{TenantID, Environment, ClusterID, Namespace, AWSAccountID, Region, ArgoCDProject, GrafanaOrgID}` is threaded through all operations for scoping and audit.

## Database Migrations

Sequential SQL files in `migrations/`. Applied via `cmd/migrate` (goose-based runner). Current range: `0001` to `0020`.

## E2E Testing

Requires Docker, kind, kubectl, helm. See `e2e/README.md`. Boot: `scripts/e2e-up.sh` then `scripts/e2e-ports.sh` then `scripts/e2e-config.sh`. Services run via `scripts/e2e-run.sh` or individually with `go run ./cmd/<service> -config e2e/config.local.json`.

## Deployment

- **Docker**: Multi-stage Dockerfiles in `docker/` for all services. Non-root containers (UID 10001), pinned Alpine 3.21, HEALTHCHECK directives.
- **Docker Compose**: `deploy/docker-compose.yml` — full local stack (gateway, tool-router, orchestrator, agent, postgres, opa, temporal, temporal-admin).
- **Helm**: `deploy/helm/carapulse/` — production Kubernetes chart with ConfigMap config, migration Job hook, security contexts, conditional resources.
- **CI/CD**: `.github/workflows/ci.yml` (test, lint, OPA test, build matrix), `.github/workflows/docker.yml` (build+push to ghcr.io).

## Implementation Status

Application code is ~90% complete — all business logic is real, no stubs. Operational infrastructure is fully implemented.

**What's fully working:** All 18+ REST endpoints, 5 Temporal workflows with rollback, 16 tool integrations, LLM planning (OpenAI/Anthropic/Codex), OPA policy framework with 7 Rego rule files, constraint enforcement, Linear approval workflow, 11 context collectors, sandbox isolation, Vault/Boundary integration, JWT/OIDC auth with JWKS caching, WebSocket/SSE streaming, Slack ChatOps, cron scheduler, audit trails, structured logging (slog), Prometheus metrics, health endpoints, graceful shutdown, Docker/Compose/Helm deployment, CI/CD pipeline.

## Config

All services share one JSON config. See `e2e/config.sample.json` for a working example. Key sections: `gateway`, `tool_router`, `orchestrator`, `storage`, `sandbox`, `context`, `connectors`, `llm`, `policy`, `chatops`, `approvals`, `scheduler`.
