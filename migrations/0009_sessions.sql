-- +goose Up
CREATE TABLE IF NOT EXISTS sessions (
  session_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  owner_actor_id TEXT,
  metadata JSONB,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE TABLE IF NOT EXISTS session_members (
  session_id TEXT NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
  member_id TEXT NOT NULL,
  role TEXT NOT NULL,
  added_at TIMESTAMP WITH TIME ZONE NOT NULL,
  PRIMARY KEY (session_id, member_id)
);

CREATE INDEX IF NOT EXISTS idx_sessions_tenant ON sessions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_session_members_session ON session_members(session_id);

-- +goose Down
DROP TABLE IF EXISTS session_members CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
