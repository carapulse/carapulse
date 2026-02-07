-- +goose Up
CREATE TABLE IF NOT EXISTS plans (
  plan_id TEXT PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL,
  trigger TEXT NOT NULL,
  summary TEXT NOT NULL,
  context_json JSONB NOT NULL,
  risk_level TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS plan_steps (
  step_id TEXT PRIMARY KEY,
  plan_id TEXT NOT NULL REFERENCES plans(plan_id),
  action TEXT NOT NULL,
  tool TEXT NOT NULL,
  input_json JSONB NOT NULL,
  preconditions_json JSONB,
  rollback_json JSONB
);

CREATE TABLE IF NOT EXISTS executions (
  execution_id TEXT PRIMARY KEY,
  plan_id TEXT NOT NULL REFERENCES plans(plan_id),
  status TEXT NOT NULL,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tool_calls (
  tool_call_id TEXT PRIMARY KEY,
  execution_id TEXT NOT NULL REFERENCES executions(execution_id),
  tool_name TEXT NOT NULL,
  input_ref TEXT,
  output_ref TEXT,
  status TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS evidence (
  evidence_id TEXT PRIMARY KEY,
  execution_id TEXT NOT NULL REFERENCES executions(execution_id),
  type TEXT NOT NULL,
  query TEXT,
  result_ref TEXT,
  link TEXT,
  collected_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS approvals (
  approval_id TEXT PRIMARY KEY,
  plan_id TEXT NOT NULL REFERENCES plans(plan_id),
  status TEXT NOT NULL,
  approver_json JSONB,
  expires_at TIMESTAMPTZ,
  source TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_events (
  event_id TEXT PRIMARY KEY,
  occurred_at TIMESTAMPTZ NOT NULL,
  actor_json JSONB NOT NULL,
  action TEXT NOT NULL,
  decision TEXT NOT NULL,
  context_json JSONB,
  evidence_refs_json JSONB,
  hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS context_nodes (
  node_id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  labels_json JSONB,
  owner_team TEXT
);

CREATE TABLE IF NOT EXISTS context_edges (
  edge_id TEXT PRIMARY KEY,
  from_node_id TEXT NOT NULL REFERENCES context_nodes(node_id),
  to_node_id TEXT NOT NULL REFERENCES context_nodes(node_id),
  relation TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_executions_plan_id ON executions(plan_id);
CREATE INDEX IF NOT EXISTS idx_tool_calls_execution_id ON tool_calls(execution_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_occurred_at ON audit_events(occurred_at);
CREATE INDEX IF NOT EXISTS idx_plans_created_at ON plans(created_at);

-- +goose Down
DROP TABLE IF EXISTS context_edges CASCADE;
DROP TABLE IF EXISTS context_nodes CASCADE;
DROP TABLE IF EXISTS audit_events CASCADE;
DROP TABLE IF EXISTS approvals CASCADE;
DROP TABLE IF EXISTS evidence CASCADE;
DROP TABLE IF EXISTS tool_calls CASCADE;
DROP TABLE IF EXISTS executions CASCADE;
DROP TABLE IF EXISTS plan_steps CASCADE;
DROP TABLE IF EXISTS plans CASCADE;
