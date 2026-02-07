-- +goose Up
ALTER TABLE runbooks ADD COLUMN IF NOT EXISTS tenant_id TEXT;
CREATE INDEX IF NOT EXISTS idx_runbooks_tenant ON runbooks(tenant_id);

-- +goose Down
DROP INDEX IF EXISTS idx_runbooks_tenant;
ALTER TABLE runbooks DROP COLUMN IF EXISTS tenant_id;
