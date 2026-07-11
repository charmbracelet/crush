-- +goose Up
-- is_new marks a file version whose file the agent created (it did not exist
-- before). On revert, such files are deleted; files that pre-existed are
-- restored to their baseline instead. Existing rows default to 0 (pre-existing)
-- so a revert restores rather than deletes them.
ALTER TABLE files ADD COLUMN is_new INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE files DROP COLUMN is_new;
