-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN workspace_cwd TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN workspace_cwd;
-- +goose StatementEnd
