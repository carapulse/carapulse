-- +goose Up
ALTER TABLE workflow_catalog ADD COLUMN IF NOT EXISTS tenant_id TEXT;
CREATE INDEX IF NOT EXISTS idx_workflow_catalog_tenant ON workflow_catalog(tenant_id);

-- +goose Down
DROP INDEX IF EXISTS idx_workflow_catalog_tenant;
ALTER TABLE workflow_catalog DROP COLUMN IF EXISTS tenant_id;
