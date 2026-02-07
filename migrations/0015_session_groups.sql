-- +goose Up
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS group_id TEXT;
CREATE INDEX IF NOT EXISTS idx_sessions_group ON sessions(group_id);

-- +goose Down
DROP INDEX IF EXISTS idx_sessions_group;
ALTER TABLE sessions DROP COLUMN IF EXISTS group_id;
