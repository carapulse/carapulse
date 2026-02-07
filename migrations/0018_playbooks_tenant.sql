-- +goose Up
ALTER TABLE playbooks ADD COLUMN IF NOT EXISTS tenant_id TEXT;
CREATE INDEX IF NOT EXISTS idx_playbooks_tenant ON playbooks(tenant_id);

-- +goose Down
DROP INDEX IF EXISTS idx_playbooks_tenant;
ALTER TABLE playbooks DROP COLUMN IF EXISTS tenant_id;
