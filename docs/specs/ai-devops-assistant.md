# AI DevOps/SRE Assistant Specification (Index)

## Purpose
Open source, autonomous DevOps/SRE assistant. Single tenant. Self hosted. Go services + Temporal + Postgres. Managed LLM API. Custom typed RPC for tools. OPA/Rego policy engine. OIDC SSO. Interfaces: API, CLI, Slack, Web UI. Change mgmt: Linear + PagerDuty. Autonomy tiers: approval required for all writes by default; low risk can be auto if configured. Tooling rule: CLI first; API only if CLI not available.

## Scope
- AWS, Vault, Kubernetes, Helm, Boundary, Argo CD, Prometheus, Thanos, Grafana, Tempo
- GitOps via GitHub/GitLab PR flow
- Observability driven diagnostics, evidence, verification
- Durable workflows with retry/rollback
- Auditable, policy constrained autonomy

## Non-goals
- Multi tenant hosted SaaS
- Full CMDB replacement
- Full human ticketing automation without approvals

## Document map
- Architecture: `docs/specs/ai-devops-assistant/architecture.md`
- Data model and config: `docs/specs/ai-devops-assistant/data-model.md`
- Interfaces (API/RPC/CLI/Slack/UI): `docs/specs/ai-devops-assistant/interfaces.md`
- Policy and security: `docs/specs/ai-devops-assistant/policy-security.md`
- Workflows and playbooks: `docs/specs/ai-devops-assistant/workflows.md`
- Integrations: `docs/specs/ai-devops-assistant/integrations.md`
- Ops and testing: `docs/specs/ai-devops-assistant/ops-testing.md`

## Key decisions
- Stack: Go, Temporal, Postgres, OTel
- LLM: managed API (OpenAI/Anthropic) + Codex subscription (`openai-codex`), routed by LLM Router service
- Tool bus: custom typed RPC, no MCP
- Policy: OPA/Rego, package `policy.assistant.v1`
- Auth: OIDC for humans, service tokens for agents
- Autonomy tiers: read auto, low auto, medium/high require approval
- Approvals: Linear issue labels + optional Slack UI

## Success criteria
- Plan, execute, verify cycle with evidence for every action
- No secret leakage to logs/LLM prompt
- Auditable decisions and approvals
- Safe defaults, bounded autonomy, rollback on failure
