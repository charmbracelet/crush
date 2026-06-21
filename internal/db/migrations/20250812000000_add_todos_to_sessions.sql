-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN todos TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN todos;
-- +goose StatementEnd
