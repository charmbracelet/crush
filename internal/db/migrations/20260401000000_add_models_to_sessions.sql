-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN models TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN models;
-- +goose StatementEnd