-- +goose Up
ALTER TABLE sessions ADD COLUMN sources TEXT;

-- +goose Down
ALTER TABLE sessions DROP COLUMN sources;
