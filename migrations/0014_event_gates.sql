-- +goose Up
CREATE TABLE IF NOT EXISTS event_gates (
  source TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  first_seen TIMESTAMP WITH TIME ZONE NOT NULL,
  last_seen TIMESTAMP WITH TIME ZONE NOT NULL,
  count INT NOT NULL,
  suppressed_until TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY (source, fingerprint)
);

CREATE INDEX IF NOT EXISTS idx_event_gates_suppressed ON event_gates(suppressed_until);

-- +goose Down
DROP TABLE IF EXISTS event_gates CASCADE;
