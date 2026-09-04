-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN agent_mode TEXT NOT NULL DEFAULT 'build';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN agent_mode;
-- +goose StatementEnd
