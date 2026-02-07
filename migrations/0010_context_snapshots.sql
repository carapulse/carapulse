-- +goose Up
CREATE TABLE IF NOT EXISTS context_snapshots (
  snapshot_id TEXT PRIMARY KEY,
  source TEXT NOT NULL,
  collected_at TIMESTAMP WITH TIME ZONE NOT NULL,
  nodes_json JSONB NOT NULL,
  edges_json JSONB NOT NULL,
  labels_json JSONB
);

CREATE INDEX IF NOT EXISTS idx_context_snapshots_collected ON context_snapshots(collected_at DESC);

-- +goose Down
DROP TABLE IF EXISTS context_snapshots CASCADE;
