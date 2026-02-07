# Implementation Plan

## Repo layout
- `cmd/gateway/` (HTTP + WS)
- `cmd/agent/` (planner, LLM router)
- `cmd/orchestrator/` (Temporal worker)
- `cmd/tool-router/` (typed RPC server)
- `cmd/sandbox-exec/` (CLI runner)
- `internal/config/` (config load/validate)
- `internal/db/` (Postgres migrations + queries)
- `internal/policy/` (OPA client + policy input/output)
- `internal/context/` (ICM refresh + graph)
- `internal/audit/` (audit append + evidence refs)
- `internal/llm/` (provider clients + redaction)
- `internal/tools/` (tool schemas + command builders)
- `internal/web/` (HTTP handlers, WS)
- `internal/workflows/` (Temporal workflows + activities)
- `internal/chatops/` (Slack)
- `internal/approvals/` (Linear watcher)
- `docs/specs/...` (already)
- `migrations/` (SQL)

## Config
- File: `internal/config/config.go`
- Structs: `Config`, `GatewayConfig`, `PolicyConfig`, `LLMConfig`, `OrchestratorConfig`, `StorageConfig`, `ConnectorsConfig`, `ChatOpsConfig`
- Load: `LoadConfig(path string) (Config, error)`
- Validate: `Validate() error`
- Env overrides optional
- LLMConfig fields: `provider`, `api_base`, `model`, `timeout_ms`, `max_output_tokens`, `redact_patterns`, `auth_profile`, `auth_path`

## DB schema + migrations
- File: `migrations/0001_init.sql`
- Tables: `plans`, `plan_steps`, `executions`, `tool_calls`, `evidence`, `approvals`, `audit_events`, `context_nodes`, `context_edges`
- Indexes: `executions(plan_id)`, `tool_calls(execution_id)`, `audit_events(occurred_at)`, `plans(created_at)`
- File: `internal/db/db.go` (`NewDB(dsn) *DB`)
- Queries: `CreatePlan`, `GetPlan`, `CreateExecution`, `UpdateExecutionStatus`, `InsertToolCall`, `InsertEvidence`, `InsertAuditEvent`, `CreateApproval`, `UpdateApprovalStatus`

## HTTP API (Gateway)
- File: `internal/web/http.go`
- Handlers: `POST /v1/plans`, `GET /v1/plans/{id}`, `POST /v1/plans/{id}:execute`, `POST /v1/approvals`, `GET /v1/executions/{id}`, `GET /v1/audit/events`, `GET /v1/context/services`, `POST /v1/hooks/alertmanager`, `POST /v1/hooks/argocd`, `POST /v1/hooks/git`
- Request structs in `internal/web/types.go` per spec
- Auth middleware: OIDC JWT verify (issuer/audience)
- Policy check: `PolicyService.EvaluatePolicy(input)`

## WebSocket + SSE
- File: `internal/web/ws.go`: events `plan.updated`, `execution.updated`, `audit.created`
- File: `internal/web/sse.go`: `GET /v1/executions/{id}/logs`

## Policy engine
- File: `internal/policy/opa.go`
- Method: `Evaluate(input PolicyInput) (PolicyDecision, error)`
- Input shape: `{actor, action, context, resources, risk, time}`
- Output: `{decision, constraints, ttl}`

## LLM Router
- File: `internal/llm/router.go`
- Providers: `OpenAIClient`, `AnthropicClient`, `CodexClient` (`openai-codex` OAuth access token via env or auth profiles)
- Redaction: `Redact(text string, patterns []string) string`
- Planner interface: `Plan(intent string, context ContextRef, evidence []Evidence) (Plan, error)`
- No tool execution from LLM

## Tool Router + Sandbox Executor
- Tool router service: `internal/tools/router.go`
- RPC interface (gRPC or HTTP+JSON): `ListTools`, `ExecuteTool`, `GetResource`, `StreamToolLogs`
- Tool schemas in `internal/tools/schemas/*.json`
- Executor: `internal/tools/sandbox.go`
- Command builder functions:
  - `BuildKubectlCmd(action string, input any) []string`
  - `BuildHelmCmd(action string, input any) []string`
  - `BuildArgoCmd(action string, input any) []string`
  - `BuildAwsCmd(action string, input any) []string`
  - `BuildVaultCmd(action string, input any) []string`
  - `BuildBoundaryCmd(action string, input any) []string`
  - `BuildGhCmd(action string, input any) []string`
- CLI-first rule enforced in router; API fallback only if CLI missing

## Context Service (ICM)
- File: `internal/context/service.go`
- Methods: `RefreshContext(ctx ContextRef)`, `GetServiceGraph(service string)`
- Pollers: Argo apps, Helm releases, AWS resources, Grafana datasources
- Watchers: K8s list/watch with bookmarks
- Store in `context_nodes` + `context_edges`

## Workflows (Temporal)
- File: `internal/workflows/workflows.go`
- Workflow functions:
  - `GitOpsDeployWorkflow(ctx workflow.Context, in DeployInput) error`
  - `HelmReleaseWorkflow(ctx workflow.Context, in HelmInput) error`
  - `ScaleServiceWorkflow(ctx workflow.Context, in ScaleInput) error`
  - `IncidentRemediationWorkflow(ctx workflow.Context, in IncidentInput) error`
  - `SecretRotationWorkflow(ctx workflow.Context, in SecretRotationInput) error`
- Activities in `internal/workflows/activities.go`:
  - `QueryPrometheusActivity`
  - `QueryTempoActivity`
  - `ArgoSyncActivity`
  - `ArgoWaitActivity`
  - `HelmStatusActivity`
  - `HelmUpgradeActivity`
  - `HelmRollbackActivity`
  - `KubectlScaleActivity`
  - `KubectlRolloutStatusActivity`
  - `CreateGrafanaAnnotationActivity`
  - `CreateLinearIssueActivity`
  - `CreatePagerDutyIncidentActivity`
  - `CreateGitPullRequestActivity`
- Retry policy: exponential backoff per activity
- Approval gate: check `approvals` table before write activities

## Approvals
- File: `internal/approvals/linear.go`
- Create issue on plan requiring approval: label `approval:pending`
- Watcher (poll or webhook) updates approval status
- Timeout task: marks expired

## ChatOps
- File: `internal/chatops/slack.go`
- Commands: `/assistant plan`, `/assistant approve`, `/assistant status`, `/assistant audit`
- Uses Gateway HTTP API

## Audit + Evidence
- File: `internal/audit/audit.go`
- Methods: `AppendAuditEvent(event AuditEvent)`, `StoreEvidence(e Evidence, blob []byte)`
- Evidence stored in object store: `evidence/{execution_id}/{evidence_id}.json`

## Helm values_ref
- `ArtifactRef` resolution helper in `internal/tools/artifacts.go`
- Supports `git_path` and `object_store`; fetch + checksum verify

## Testing
- Unit:
  - `TestPolicyEvaluate` (tier decisions)
  - `TestRedaction` (no secret leakage)
  - `TestBuildHelmCmd` / `TestBuildKubectlCmd`
- Integration (optional in CI):
  - kind + Argo CD + Prometheus + Tempo + Vault dev
- E2E:
  - GitOps deploy -> verify -> annotation
  - Helm upgrade -> verify -> rollback
  - Alert -> plan -> approve -> execute -> verify
  - Approval timeout -> denied

## Rollout
- Stage 0: read-only
- Stage 1: approvals for all writes
- Stage 2: enable low-risk auto per config
- Stage 3: expand tool catalog
