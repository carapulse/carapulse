-- +goose Up
CREATE TABLE IF NOT EXISTS schedules (
  schedule_id TEXT PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL,
  cron TEXT NOT NULL,
  context_json JSONB NOT NULL,
  summary TEXT NOT NULL,
  intent TEXT NOT NULL,
  constraints_json JSONB,
  trigger TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  last_run_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_schedules_enabled ON schedules(enabled);

-- +goose Down
DROP TABLE IF EXISTS schedules CASCADE;
