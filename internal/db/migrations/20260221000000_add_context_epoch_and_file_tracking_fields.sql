-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN context_epoch INTEGER NOT NULL DEFAULT 0;

ALTER TABLE read_files ADD COLUMN last_read_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE read_files ADD COLUMN last_read_mtime_ns INTEGER NOT NULL DEFAULT 0;
ALTER TABLE read_files ADD COLUMN last_read_size INTEGER NOT NULL DEFAULT 0;
ALTER TABLE read_files ADD COLUMN last_included_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE read_files ADD COLUMN last_included_mtime_ns INTEGER NOT NULL DEFAULT 0;
ALTER TABLE read_files ADD COLUMN last_included_size INTEGER NOT NULL DEFAULT 0;
ALTER TABLE read_files ADD COLUMN last_included_epoch INTEGER NOT NULL DEFAULT 0;

UPDATE read_files
SET last_read_at = read_at
WHERE read_at > 0 AND last_read_at = 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN context_epoch;

ALTER TABLE read_files DROP COLUMN last_included_epoch;
ALTER TABLE read_files DROP COLUMN last_included_size;
ALTER TABLE read_files DROP COLUMN last_included_mtime_ns;
ALTER TABLE read_files DROP COLUMN last_included_at;
ALTER TABLE read_files DROP COLUMN last_read_size;
ALTER TABLE read_files DROP COLUMN last_read_mtime_ns;
ALTER TABLE read_files DROP COLUMN last_read_at;
-- +goose StatementEnd
