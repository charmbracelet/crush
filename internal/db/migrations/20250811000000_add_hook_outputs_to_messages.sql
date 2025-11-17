-- +goose Up
ALTER TABLE messages ADD COLUMN hook_outputs TEXT DEFAULT '[]' NOT NULL;

-- +goose Down
ALTER TABLE messages DROP COLUMN hook_outputs;
