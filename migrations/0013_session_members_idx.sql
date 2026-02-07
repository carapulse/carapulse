-- +goose Up
CREATE INDEX IF NOT EXISTS idx_session_members_member ON session_members(member_id);

-- +goose Down
DROP INDEX IF EXISTS idx_session_members_member;
