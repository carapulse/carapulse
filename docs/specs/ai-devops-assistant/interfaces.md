# Interfaces

## HTTP API (Gateway)
All requests require OIDC auth (Bearer). JSON only.

### Endpoints
- `POST /v1/plans` -> PlanCreateResponse
- `GET /v1/plans/{plan_id}` -> Plan
- `POST /v1/plans/{plan_id}:execute` -> Execution
- `POST /v1/approvals` -> Approval
- `GET /v1/executions/{execution_id}` -> Execution
- `GET /v1/audit/events` -> AuditEvent[]
- `GET /v1/context/services` -> Service[]
- `POST /v1/hooks/alertmanager` -> HookAck
- `POST /v1/hooks/argocd` -> HookAck
- `POST /v1/hooks/git` -> HookAck

### Request/response shapes
```yaml
PlanCreateRequest:
  summary: string
  trigger: enum[manual,alert,webhook,scheduled]
  context: ContextRef
  intent: string
  constraints: object

PlanCreateResponse:
  plan_id: string
  created_at: timestamp
  plan: Plan

Plan:
  plan_id: string
  created_at: timestamp
  summary: string
  trigger: string
  intent: string
  context: ContextRef
  constraints: object|null
  risk_level: enum[read,low,medium,high]
  plan_text: string|null
  steps: [PlanStep] # optional
  approvals: [Approval] # optional

PlanExecuteRequest:
  plan_id: string
  approval_token: string|null

ApprovalCreateRequest:
  plan_id: string
  approver_note: string
  status: enum[approved,denied]

AuditQueryRequest:
  from: timestamp
  to: timestamp
  actor_id: string|null
  action: string|null
  decision: string|null

HookAck:
  received: bool
  event_id: string
```

## Internal RPC (Tool Router)
Typed RPC between Workflow and Tool Router (gRPC or HTTP+JSON with strict schemas).

```proto
service ToolRouter {
  rpc ListTools(ListToolsRequest) returns (ListToolsResponse);
  rpc ExecuteTool(ExecuteToolRequest) returns (ExecuteToolResponse);
  rpc StreamToolLogs(StreamToolLogsRequest) returns (stream ToolLogChunk);
  rpc GetResource(GetResourceRequest) returns (GetResourceResponse);
}

message ExecuteToolRequest {
  string tool_name = 1;
  string action = 2;
  bytes input_json = 3;
  ContextRef context = 4;
  string execution_id = 5;
}

message ExecuteToolResponse {
  string tool_call_id = 1;
  string status = 2;
  string output_ref = 3;
}
```

## Tool Router HTTP
- `GET /v1/tools/logs?tool_call_id=...` (SSE log stream)

## CLI
- `assistantctl plan create --summary ... --context ...`
- `assistantctl plan approve --plan-id ... --status approved|denied`
- `assistantctl exec logs --execution-id ...`
- `assistantctl context refresh --service ...`
- `assistantctl policy test --input ...`
- `assistantctl llm auth import --codex-auth <path> [--auth-path <path>] [--profile-id <id>]`
- `assistantctl llm auth import --openclaw-auth <path> [--provider <name>] [--auth-path <path>] [--profile-id <id>]`
- `assistantctl llm auth login [--codex-auth <path>] [--auth-path <path>] [--profile-id <id>]`

## Slack ChatOps
- `/assistant plan <intent>` -> returns plan draft + approve link
- `/assistant approve <plan_id>` -> approval
- `/assistant status <execution_id>` -> status + evidence links
- `/assistant audit <plan_id>` -> audit summary

## Web UI
Pages:
- Dashboard (active alerts, open plans, running executions)
- Plans (list/detail, approval UI)
- Executions (timeline + evidence)
- Context Graph (ICM view)
- Policies (bundles, last updated)
- Integrations (status, auth)
- Audit Log (search + export)


## WebSocket
- `wss://.../v1/ws`
- Events: `plan.updated`, `execution.updated`, `audit.created`
- Payloads: `{ event, data, ts }`

## Streaming logs
- `GET /v1/executions/{execution_id}/logs` (SSE)
- Filters: `tool_call_id`, `level`
