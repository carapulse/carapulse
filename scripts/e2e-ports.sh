#!/usr/bin/env bash
set -euo pipefail

pids=()

port_forward() {
  local ns="$1"
  local svc="$2"
  local local_port="$3"
  local remote_port="$4"
  kubectl -n "$ns" port-forward "svc/$svc" "${local_port}:${remote_port}" >/dev/null 2>&1 &
  pids+=("$!")
}

port_forward argocd argocd-server 8083 443
port_forward monitoring kube-prometheus-stack-prometheus 9090 9090
port_forward monitoring kube-prometheus-stack-alertmanager 9093 9093
port_forward monitoring kube-prometheus-stack-grafana 3000 80
port_forward observability tempo 3200 3200

trap 'kill "${pids[@]}"' EXIT
echo "port-forwards active: argocd=8083 prom=9090 alert=9093 grafana=3000 tempo=3200"
wait
