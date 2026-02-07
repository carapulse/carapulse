# Policy and security

## Autonomy tiers
| Tier | Actions | Default | Approval |
|---|---|---|---|
| Read | inventory, metrics, traces, status | allow | none |
| Low | Argo dry-run, scale within limits | require approval (default); can be auto if configured | Linear/Slack/UI |
| Medium | Argo sync, rollout restart | require approval | Linear/Slack/UI |
| High | destructive changes, IAM edits, network changes | require approval + break-glass | Linear + SRE on-call |

## Policy engine
- Engine: OPA/Rego
- Package: `policy.assistant.v1`
- Input shape: `{ actor, action, context, resources, risk, time }`
- Output shape: `{ decision, constraints, ttl }`
- Decision values: `allow`, `deny`, `require_approval`
- `require_approval` triggers approval creation and blocks execution until approved

## Approval flow
- Low/medium/high actions create a Linear issue labeled `approval:pending` by default
- Approver changes label to `approval:approved` or `approval:denied`
- Gateway watches Linear, updates Approval record
- Timeout: 24h default, then `expired`

## Secrets handling
- Vault Agent auto-auth for services
- Secrets only in memory, never in logs
- LLM prompt redaction via `llm.redact_patterns`
- Tool outputs stored in object store, access via signed URLs

## Sandbox executor
- Unprivileged container
- Read-only root FS, tmpfs for temp
- Egress allowlist by tool category
- No host mount by default

## Threat model summary
- Prompt injection: strict tool gating, no free-form tool execution
- Excessive agency: tiered autonomy + approvals
- Sensitive data disclosure: redaction, no secret output
- Insecure plugin design: typed RPC schemas + validation


## Constraint enforcement
- Max targets per action (default 50 resources)
- Time windows for risky actions (maintenance windows)
- Environment locks (prod require approval)

## Audit retention
- Audit events retained 365 days
- Evidence files retained 90 days by default
- Export path: CSV/JSON via UI and API
