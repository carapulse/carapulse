-- +goose Up
ALTER TABLE evidence ADD COLUMN IF NOT EXISTS external_ids JSONB;

-- +goose Down
ALTER TABLE evidence DROP COLUMN IF EXISTS external_ids;
