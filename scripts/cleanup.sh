#!/usr/bin/env bash
set -euo pipefail

# Cleanup Carapulse E2E environment
# Usage: ./scripts/cleanup.sh

echo "Cleaning up Carapulse E2E environment..."

# Stop docker-compose services
if [ -f "e2e/docker-compose.yml" ]; then
  echo "Stopping docker-compose services..."
  docker compose -f e2e/docker-compose.yml down -v 2>/dev/null || true
fi

# Delete kind cluster
if kind get clusters 2>/dev/null | grep -q carapulse-e2e; then
  echo "Deleting kind cluster..."
  kind delete cluster --name carapulse-e2e
fi

# Clean up generated config
if [ -f "e2e/config.local.json" ]; then
  echo "Removing generated config..."
  rm -f e2e/config.local.json
fi

# Kill any tmux sessions
if tmux has-session -t carapulse-e2e 2>/dev/null; then
  echo "Killing tmux session..."
  tmux kill-session -t carapulse-e2e
fi

echo "Cleanup complete."
