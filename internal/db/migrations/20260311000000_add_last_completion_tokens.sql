-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN last_completion_tokens INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN last_completion_tokens;
-- +goose StatementEnd
