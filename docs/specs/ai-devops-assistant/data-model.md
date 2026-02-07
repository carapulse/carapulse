# Data model

## Canonical objects (YAML shape)
```yaml
Actor:
  id: string
  type: enum[user, service, agent]
  roles: [string]
  tenant_id: string
  display_name: string

ContextRef:
  tenant_id: string
  environment: enum[dev,stage,prod]
  cluster_id: string
  namespace: string
  aws_account_id: string
  region: string
  argocd_project: string
  grafana_org_id: string

ResourceRef:
  kind: string
  id: string
  name: string
  labels: map[string]string
  owner_team: string

Plan:
  plan_id: string
  created_at: timestamp
  trigger: enum[manual,alert,webhook,scheduled]
  summary: string
  intent: string
  context: ContextRef
  risk_level: enum[read,low,medium,high]
  constraints: object|null
  plan_text: string|null
  steps: [PlanStep]
  approvals: [Approval]

PlanStep:
  step_id: string
  action: string
  tool: string
  input: object
  preconditions: [string]
  rollback: RollbackSpec
  evidence_required: [EvidenceRequirement]

Execution:
  execution_id: string
  plan_id: string
  status: enum[pending,running,failed,succeeded,rolled_back,cancelled]
  started_at: timestamp
  completed_at: timestamp
  tool_calls: [ToolCall]

ToolCall:
  tool_call_id: string
  tool_name: string
  input_ref: string
  output_ref: string
  status: enum[pending,running,failed,succeeded]
  sandbox_id: string
  started_at: timestamp
  completed_at: timestamp

Evidence:
  evidence_id: string
  type: enum[promql,traceql,argocd,k8s,cloudtrail,git,log]
  query: string
  result_ref: string
  link: string
  collected_at: timestamp

Approval:
  approval_id: string
  plan_id: string
  status: enum[pending,approved,denied,expired]
  approver: Actor
  expires_at: timestamp
  source: enum[linear,ui,cli,slack]

AuditEvent:
  event_id: string
  occurred_at: timestamp
  actor: Actor
  action: string
  decision: enum[allow,deny,require_approval]
  context: ContextRef
  evidence_refs: [string]
  hash: string
```

## Config schema
```yaml
Config:
  gateway:
    http_addr: string
    ws_addr: string
    oidc_issuer: string
    oidc_client_id: string
  policy:
    opa_url: string
    policy_package: string
  llm:
    provider: enum[openai,anthropic,openai-codex]
    api_base: string
    model: string
    timeout_ms: int
    max_output_tokens: int
    redact_patterns: [string]
    auth_profile: string
    auth_path: string
  orchestrator:
    temporal_addr: string
    namespace: string
    task_queue: string
  storage:
    postgres_dsn: string
    object_store:
      endpoint: string
      bucket: string
  connectors:
    aws: { role_arn: string }
    vault: { addr: string }
    k8s: { kubeconfig_path: string }
    argocd: { addr: string }
    grafana: { addr: string }
    tempo: { addr: string }
  chatops:
    slack_bot_token: string
    slack_signing_secret: string

Example config (OpenAI API key):
```yaml
llm:
  provider: openai
  api_base: https://api.openai.com
  model: gpt-4o-mini
  timeout_ms: 5000
  max_output_tokens: 512
  redact_patterns: ["(?i)password=.*"]
```
Env: `OPENAI_API_KEY`

Example config (Codex subscription):
```yaml
llm:
  provider: openai-codex
  model: gpt-4o-mini
  auth_profile: openai-codex:default
  auth_path: ~/.carapulse/auth-profiles.json
```
Env: `OPENAI_ACCESS_TOKEN` or import via `assistantctl llm auth import`; optional `CARAPULSE_AUTH_PROFILE`, `CARAPULSE_AUTH_PATH`.
```

## Postgres schema (logical)
- `plans(plan_id pk, created_at, trigger, summary, context_json, risk_level, intent, constraints_json, plan_text)`
- `plan_steps(step_id pk, plan_id fk, action, tool, input_json, preconditions_json, rollback_json)`
- `executions(execution_id pk, plan_id fk, status, started_at, completed_at)`
- `tool_calls(tool_call_id pk, execution_id fk, tool_name, input_ref, output_ref, status)`
- `evidence(evidence_id pk, execution_id fk, type, query, result_ref, link, collected_at)`
- `approvals(approval_id pk, plan_id fk, status, approver_json, expires_at, source)`
- `audit_events(event_id pk, occurred_at, actor_json, action, decision, context_json, evidence_refs_json, hash)`
- `context_nodes(node_id pk, kind, name, labels_json, owner_team)`
- `context_edges(edge_id pk, from_node_id, to_node_id, relation)`

## Object store keys
- `evidence/{execution_id}/{evidence_id}.json`
- `tool-output/{execution_id}/{tool_call_id}.json`
- `audit/{date}/{event_id}.json`


## Additional shapes
```yaml
RiskScore:
  level: enum[low,medium,high]
  reasons: [string]
  blast_radius: enum[namespace,cluster,account]

ApprovalRequirement:
  tier: enum[medium,high]
  approvers: [string]
  ttl_hours: int

RollbackSpec:
  type: enum[argo,scale,custom]
  steps: [string]

EvidenceRequirement:
  type: enum[promql,traceql,argocd,k8s,cloudtrail,git]
  query: string

Service:
  service_id: string
  name: string
  owner_team: string
  environments: [string]
  argo_apps: [string]
  helm_releases: [string]
  promql_queries: [string]
  traceql_queries: [string]
  grafana_dashboards: [string]

ArtifactRef:
  kind: enum[git_path,object_store,inline]
  ref: string
  sha: string|null
```
