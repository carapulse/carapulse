-- +goose Up
CREATE TABLE IF NOT EXISTS alert_events (
  alert_id TEXT PRIMARY KEY,
  fingerprint TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TIMESTAMP WITH TIME ZONE,
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
  payload_json JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alert_events_fingerprint ON alert_events(fingerprint);

-- +goose Down
DROP TABLE IF EXISTS alert_events CASCADE;
