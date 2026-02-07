# E2E local

Goal: run full gateway + tool-router + orchestrator + temporal, against kind + Argo CD + Prometheus/Grafana/Alertmanager + Tempo.

## Prereqs
- CLI: docker, kind, kubectl, helm, curl, jq
- Optional: argocd CLI (for app login/create, token)
- Optional: tmux (run 3 services)

## Boot
1. `./scripts/e2e-up.sh` (docker compose + kind + installs)
2. `./scripts/e2e-ports.sh` (keep running; port-forwards)
3. `./scripts/e2e-config.sh` -> writes `e2e/config.local.json`
4. Start services (3 terminals or tmux):
   - `./scripts/e2e-run.sh` (tmux) OR
   - `go run ./cmd/tool-router -config e2e/config.local.json`
   - `go run ./cmd/orchestrator -config e2e/config.local.json`
   - `go run ./cmd/gateway -config e2e/config.local.json`

## Config
- Base template: `e2e/config.sample.json`
- Generated file: `e2e/config.local.json`
- Token sources used by `scripts/e2e-config.sh`:
  - Grafana API key via Grafana admin (port-forward 3000)
  - ArgoCD token via `argocd` CLI (port-forward 8083)
- Manual tokens:
  - `E2E_GRAFANA_TOKEN=<token> ./scripts/e2e-config.sh`
  - `E2E_ARGOCD_TOKEN=<token> ./scripts/e2e-config.sh`
- Override context fields via env or `--context @file` in `cmd/e2e`:
  - Required: tenant_id, environment, cluster_id, namespace, aws_account_id, region, argocd_project, grafana_org_id

## Service endpoints
Configurable via:
- `scripts/e2e-ports.sh` (local port-forwards)
- `e2e/config.local.json` (`connectors.*.addr`)
Defaults when using provided scripts:
- ArgoCD: https://127.0.0.1:8083
- Prometheus: http://127.0.0.1:9090
- Alertmanager: http://127.0.0.1:9093
- Grafana: http://127.0.0.1:3000
- Tempo: http://127.0.0.1:3200

## Run flows
Example scale flow:
`go run ./cmd/e2e --workflows scale_service --scale-resource deploy/e2e-app --scale-replicas 2`

Example helm flow:
`go run ./cmd/e2e --workflows helm_release --helm-release e2e-nginx --helm-namespace e2e --helm-chart bitnami/nginx`

Example gitops flow (requires ArgoCD app):
`go run ./cmd/e2e --workflows gitops_deploy --argocd-app e2e-app`

## E2E runner flags/env
Common env:
- `E2E_GATEWAY_URL` (default http://127.0.0.1:8080)
- `E2E_TOKEN` (gateway auth token if enabled)
- `E2E_TENANT_ID`, `E2E_ENVIRONMENT`, `E2E_CLUSTER_ID`, `E2E_NAMESPACE`
- `E2E_AWS_ACCOUNT_ID`, `E2E_REGION`, `E2E_ARGOCD_PROJECT`, `E2E_GRAFANA_ORG_ID`
- `E2E_TIMEOUT`, `E2E_WORKFLOWS`
Common flags:
- `--context @file.json` (full context override)
- `--workflows gitops_deploy,helm_release,scale_service`

## Optional: create ArgoCD app
If `argocd` CLI is installed (and port-forward active):
`argocd app create e2e-app --repo https://github.com/argoproj/argocd-example-apps.git --path guestbook --dest-server https://kubernetes.default.svc --dest-namespace e2e --project default`

## Notes
- Gateway strict context: tenant/environment/cluster/namespace/aws_account/region/argocd_project/grafana_org required.
- Grafana token + ArgoCD token must be set in `e2e/config.local.json`.
- `scripts/e2e-ports.sh` uses port-forwards; keep it running.

## Cleanup
- `kind delete cluster --name carapulse-e2e`
- `docker compose -f e2e/docker-compose.yml down -v`
