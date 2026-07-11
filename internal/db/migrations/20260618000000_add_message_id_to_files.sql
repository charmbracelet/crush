-- +goose Up
ALTER TABLE files ADD COLUMN message_id TEXT;

-- +goose Down
ALTER TABLE files DROP COLUMN message_id;
