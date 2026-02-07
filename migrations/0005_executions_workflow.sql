-- +goose Up
ALTER TABLE executions ADD COLUMN IF NOT EXISTS workflow_id TEXT;

-- +goose Down
ALTER TABLE executions DROP COLUMN IF EXISTS workflow_id;
