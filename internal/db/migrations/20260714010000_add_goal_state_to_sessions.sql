-- +goose Up
ALTER TABLE sessions ADD COLUMN goal_state TEXT;

-- +goose Down
ALTER TABLE sessions DROP COLUMN goal_state;
