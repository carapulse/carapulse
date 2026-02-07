-- +goose Up
CREATE TABLE IF NOT EXISTS runbooks (
  runbook_id TEXT PRIMARY KEY,
  service TEXT NOT NULL,
  name TEXT NOT NULL,
  version INTEGER NOT NULL,
  tags JSONB,
  body TEXT,
  spec_json JSONB,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS runbooks_service_name_idx
  ON runbooks(service, name);

-- +goose Down
DROP TABLE IF EXISTS runbooks CASCADE;
