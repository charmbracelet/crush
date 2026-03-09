-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN collaboration_mode TEXT NOT NULL DEFAULT 'default';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN collaboration_mode;
-- +goose StatementEnd
