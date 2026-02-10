-- +goose Up
ALTER TABLE approvals ADD COLUMN approved_hash TEXT;

-- +goose Down
ALTER TABLE approvals DROP COLUMN IF EXISTS approved_hash;
