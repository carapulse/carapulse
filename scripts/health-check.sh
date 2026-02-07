#!/usr/bin/env bash
set -euo pipefail

# Health check for all Carapulse services
# Usage: ./scripts/health-check.sh [gateway-url]

GATEWAY_URL="${1:-http://localhost:8080}"
TOOL_ROUTER_URL="${2:-http://localhost:8081}"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

check() {
  local name="$1"
  local url="$2"
  if curl -sf --max-time 5 "$url" > /dev/null 2>&1; then
    printf "${GREEN}OK${NC}  %s (%s)\n" "$name" "$url"
  else
    printf "${RED}FAIL${NC} %s (%s)\n" "$name" "$url"
    FAILED=1
  fi
}

FAILED=0

echo "Carapulse Health Check"
echo "======================"
check "Gateway healthz" "${GATEWAY_URL}/healthz"
check "Gateway readyz"  "${GATEWAY_URL}/readyz"
check "Gateway metrics" "${GATEWAY_URL}/metrics"
check "Tool Router healthz" "${TOOL_ROUTER_URL}/healthz"
check "Tool Router readyz"  "${TOOL_ROUTER_URL}/readyz"

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "All services healthy."
else
  echo "Some services are unhealthy!"
  exit 1
fi
