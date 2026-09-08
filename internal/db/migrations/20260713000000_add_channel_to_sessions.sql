-- +goose Up
ALTER TABLE sessions ADD COLUMN channel TEXT;

-- +goose Down
ALTER TABLE sessions DROP COLUMN channel;
