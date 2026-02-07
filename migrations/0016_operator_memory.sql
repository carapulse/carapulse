-- +goose Up
CREATE TABLE IF NOT EXISTS operator_memory (
  memory_id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  tags JSONB,
  metadata JSONB,
  owner_actor_id TEXT,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_operator_memory_tenant ON operator_memory(tenant_id);

-- +goose Down
DROP TABLE IF EXISTS operator_memory CASCADE;
