# Carapulse

Open-source, autonomous DevOps/SRE assistant. Single-tenant, self-hosted. Plans, executes, and verifies infrastructure operations with approval gates, policy enforcement, and audit trails.

**Stack:** Go + Temporal + Postgres + OPA

## Features

- **Plan-Execute-Verify loop** — LLM-powered planning with human approval gates
- **16 tool integrations** — kubectl, helm, argocd, aws, vault, boundary, prometheus, grafana, and more
- **5 durable workflows** — GitOps deploy, Helm release, scale service, incident remediation, secret rotation
- **Policy enforcement** — OPA/Rego rules for read/write/break-glass authorization
- **Context awareness** — 11 collectors (K8s, Helm, ArgoCD, Prometheus, Thanos, Alertmanager, Grafana, Tempo, AWS)
- **Approval workflow** — Linear-based approval with configurable auto-approve for low-risk operations
- **Real-time streaming** — WebSocket and SSE event delivery
- **ChatOps** — Slack integration via the agent service
- **Audit trail** — Every action logged with actor, decision, and context
- **Observability** — Structured logging (slog), Prometheus metrics on all services

## Architecture

```
                    ┌─────────────┐
                    │   Gateway   │ :8080 HTTP API + SSE/WS
                    │             │ :8082 Canvas
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
       ┌──────┴──────┐ ┌──┴───┐ ┌──────┴──────┐
       │ Tool Router │ │  OPA │ │ Orchestrator│
       │   :8081     │ │      │ │  (Temporal) │
       └─────────────┘ └──────┘ └─────────────┘
              │
    ┌─────────┼──────────┐
    │         │          │
  kubectl   helm    argocd ...
```

Six binaries:
- **gateway** — HTTP API server with all REST endpoints
- **tool-router** — Tool execution proxy (CLI-first with API fallback)
- **orchestrator** — Temporal worker for durable workflows
- **agent** — Slack ChatOps bridge
- **assistantctl** — CLI client
- **sandbox-exec** — Standalone sandboxed command runner

## Quick Start

### Prerequisites

- Go 1.23+
- Docker and Docker Compose
- PostgreSQL 15+

### Local Development (Docker Compose)

```bash
# Start all services
cd deploy && docker compose up -d

# Check health
curl http://localhost:8080/healthz
curl http://localhost:8081/healthz

# Create a plan
curl -X POST http://localhost:8080/v1/plans \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <token>' \
  -d '{"summary":"Scale nginx","trigger":"manual","context":{"tenant_id":"default","environment":"dev"}}'
```

### Build from Source

```bash
go build ./...
go test ./...
```

### E2E Testing

```bash
# Boot E2E environment (Docker + kind cluster)
./scripts/e2e-up.sh
./scripts/e2e-ports.sh &
./scripts/e2e-config.sh

# Run services
./scripts/e2e-run.sh

# Cleanup
./scripts/cleanup.sh
```

## Configuration

All services share one JSON config file. See `e2e/config.sample.json` for a complete example.

Key sections: `gateway`, `tool_router`, `orchestrator`, `storage`, `sandbox`, `context`, `connectors`, `llm`, `policy`, `chatops`, `approvals`, `scheduler`.

## Deployment

### Helm

```bash
helm install carapulse deploy/helm/carapulse/ \
  --namespace carapulse \
  --create-namespace \
  -f my-values.yaml
```

### Docker Compose

```bash
cd deploy && docker compose up -d
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/plans` | Create a plan |
| GET | `/v1/plans/:id` | Get plan details |
| POST | `/v1/plans/:id/execute` | Execute a plan |
| POST | `/v1/approvals` | Approve/deny a plan |
| GET | `/v1/executions/:id` | Get execution status |
| GET | `/v1/executions/:id/logs` | Stream execution logs (SSE) |
| POST/GET | `/v1/sessions` | Manage sessions |
| POST/GET | `/v1/playbooks` | Manage playbooks |
| POST/GET | `/v1/runbooks` | Manage runbooks |
| GET | `/v1/workflows` | List workflow catalog |
| POST | `/v1/workflows/:name/start` | Start a workflow |
| POST/GET | `/v1/schedules` | Manage schedules |
| GET | `/v1/context/services` | List context services |
| GET | `/v1/context/graph` | Get service graph |
| POST | `/v1/hooks/alertmanager` | Alertmanager webhook |
| GET | `/v1/audit/events` | Query audit trail |
| GET | `/healthz` | Liveness check |
| GET | `/readyz` | Readiness check |
| GET | `/metrics` | Prometheus metrics |

## Development

```bash
# Run tests
go test ./...

# Run with race detector
go test -race ./...

# Run linter
golangci-lint run ./...

# Run OPA policy tests
opa test policies/ -v
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
