#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
E2E_DIR="$ROOT/e2e"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing $1"
    exit 1
  fi
}

need_cmd docker
need_cmd kind
need_cmd kubectl
need_cmd helm
need_cmd curl
need_cmd jq
need_cmd go

if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE="docker-compose"
else
  echo "missing docker compose"
  exit 1
fi

$COMPOSE -f "$E2E_DIR/docker-compose.yml" up -d

POSTGRES_ID="$($COMPOSE -f "$E2E_DIR/docker-compose.yml" ps -q postgres || true)"
if [ -n "${POSTGRES_ID:-}" ]; then
  echo "waiting postgres health..."
  for _ in $(seq 1 60); do
    status="$(docker inspect --format '{{.State.Health.Status}}' "$POSTGRES_ID" 2>/dev/null || true)"
    if [ "$status" = "healthy" ]; then
      break
    fi
    sleep 1
  done
  if [ "$(docker inspect --format '{{.State.Health.Status}}' "$POSTGRES_ID" 2>/dev/null || true)" != "healthy" ]; then
    echo "postgres not healthy"
    exit 1
  fi
fi

go run ./cmd/migrate \
  -dsn "postgres://carapulse:carapulse@127.0.0.1:5432/carapulse?sslmode=disable" \
  -action up

mkdir -p "$E2E_DIR/workspace"

if ! kind get clusters | grep -q "^carapulse-e2e$"; then
  kind create cluster --name carapulse-e2e
fi

kubectl get ns argocd >/dev/null 2>&1 || kubectl create ns argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null 2>&1 || true
helm repo add grafana https://grafana.github.io/helm-charts >/dev/null 2>&1 || true
helm repo add bitnami https://charts.bitnami.com/bitnami >/dev/null 2>&1 || true
helm repo update >/dev/null 2>&1 || true

helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  -n monitoring --create-namespace
helm upgrade --install tempo grafana/tempo \
  -n observability --create-namespace --set tempo.searchEnabled=true

kubectl get ns e2e >/dev/null 2>&1 || kubectl create ns e2e
kubectl -n e2e get deploy e2e-app >/dev/null 2>&1 || kubectl -n e2e create deployment e2e-app --image=nginx:1.25
helm upgrade --install e2e-nginx bitnami/nginx -n e2e --create-namespace

echo "e2e deps ready"
echo "next: ./scripts/e2e-ports.sh"
