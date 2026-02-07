#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CFG="${1:-$ROOT/e2e/config.local.json}"

if [[ ! -f "$CFG" ]]; then
  echo "missing config: $CFG"
  exit 1
fi

if command -v tmux >/dev/null 2>&1; then
  SESSION="carapulse-e2e"
  tmux has-session -t "$SESSION" >/dev/null 2>&1 && tmux kill-session -t "$SESSION"
  tmux new-session -d -s "$SESSION" -n tool-router "cd $ROOT && go run ./cmd/tool-router -config $CFG"
  tmux new-window -t "$SESSION" -n orchestrator "cd $ROOT && go run ./cmd/orchestrator -config $CFG"
  tmux new-window -t "$SESSION" -n gateway "cd $ROOT && go run ./cmd/gateway -config $CFG"
  echo "tmux session $SESSION ready"
  echo "attach: tmux attach -t $SESSION"
  exit 0
fi

echo "tmux not found; run in separate terminals:"
echo "1) go run ./cmd/tool-router -config $CFG"
echo "2) go run ./cmd/orchestrator -config $CFG"
echo "3) go run ./cmd/gateway -config $CFG"
