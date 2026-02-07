# Ops and testing

## Observability
- Logs: structured JSON, trace IDs
- Metrics: request latency, plan counts, policy denies, tool error rate
- Traces: end to end for plan -> execute

## SLOs
- Plan create p95 < 10s
- Execute start p95 < 30s
- Audit write success >= 99.9%
- Workflow completion success >= 98%

## Backup and restore
- Postgres PITR
- Object store lifecycle and versioning
- Policy bundles stored in Git

## Test matrix
Unit:
- Policy evaluation for risk tiers
- Input validation for tool router
- Redaction of secrets before LLM prompt

Integration:
- kind + Argo CD + Prometheus + Tempo + Vault dev
- Tool calls via sandbox
- OIDC auth flow

E2E:
- GitOps deploy -> verify -> annotation
- Helm upgrade -> verify -> rollback
- Alert -> diagnose -> plan -> approve -> execute -> verify
- Approval timeout -> denied flow
- Local harness: `scripts/e2e-up.sh`, `scripts/e2e-ports.sh`, `scripts/e2e-config.sh`, `scripts/e2e-run.sh`, `cmd/e2e`

## CI gates
- Lint, typecheck, unit tests
- Integration test suite
- E2E smoke tests


## Rollout plan
- Stage 0: read only mode
- Stage 1: all write actions require approval
- Stage 2: optionally enable low-risk auto for selected services
- Stage 3: expand action catalog

## Runbooks
- Policy engine down -> fail closed
- Temporal outage -> pause new executions
- LLM provider outage -> degrade to read only
