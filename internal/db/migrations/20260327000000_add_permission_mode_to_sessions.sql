-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN permission_mode TEXT NOT NULL DEFAULT 'auto';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN permission_mode;
-- +goose StatementEnd
