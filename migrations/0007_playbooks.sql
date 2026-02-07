-- +goose Up
CREATE TABLE IF NOT EXISTS playbooks (
  playbook_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  version INT NOT NULL,
  tags JSONB,
  spec_json JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS playbooks_name_idx ON playbooks(name);

-- +goose Down
DROP TABLE IF EXISTS playbooks CASCADE;
