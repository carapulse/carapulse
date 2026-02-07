-- +goose Up
CREATE TABLE IF NOT EXISTS workflow_catalog (
  workflow_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  version INT NOT NULL,
  spec_json JSONB NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_workflow_catalog_name_version ON workflow_catalog(name, version);

-- +goose Down
DROP TABLE IF EXISTS workflow_catalog CASCADE;
