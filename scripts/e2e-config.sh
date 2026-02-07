#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMPLATE="$ROOT/e2e/config.sample.json"
OUT="$ROOT/e2e/config.local.json"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing $1"
    exit 1
  fi
}

need_cmd kubectl
need_cmd jq
need_cmd curl

GRAFANA_TOKEN="${E2E_GRAFANA_TOKEN:-}"
if [[ -z "$GRAFANA_TOKEN" ]]; then
  if kubectl -n monitoring get secret kube-prometheus-stack-grafana >/dev/null 2>&1; then
    GRAFANA_PASS="$(kubectl -n monitoring get secret kube-prometheus-stack-grafana -o jsonpath='{.data.admin-password}' | base64 -d)"
    GRAFANA_TOKEN="$(curl -s -u "admin:${GRAFANA_PASS}" \
      -H "Content-Type: application/json" \
      -d '{"name":"carapulse-e2e","role":"Admin"}' \
      http://127.0.0.1:3000/api/auth/keys | jq -r '.key')"
  fi
fi

ARGOCD_TOKEN="${E2E_ARGOCD_TOKEN:-}"
if [[ -z "$ARGOCD_TOKEN" ]] && command -v argocd >/dev/null 2>&1; then
  if kubectl -n argocd get secret argocd-initial-admin-secret >/dev/null 2>&1; then
    ARGOCD_PASS="$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d)"
    argocd login 127.0.0.1:8083 --username admin --password "$ARGOCD_PASS" --insecure >/dev/null 2>&1 || true
    ARGOCD_TOKEN="$(argocd account generate-token --account admin 2>/dev/null || true)"
  fi
fi

if [[ -z "$GRAFANA_TOKEN" ]]; then
  echo "missing grafana token (set E2E_GRAFANA_TOKEN)"
  exit 1
fi
if [[ -z "$ARGOCD_TOKEN" ]]; then
  echo "missing argocd token (set E2E_ARGOCD_TOKEN or install argocd CLI)"
  exit 1
fi

jq --arg grafana "$GRAFANA_TOKEN" --arg argocd "$ARGOCD_TOKEN" \
  '.connectors.grafana.token=$grafana | .connectors.argocd.token=$argocd' \
  "$TEMPLATE" > "$OUT"

echo "wrote $OUT"
