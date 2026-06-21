-- +goose Up
-- +goose StatementBegin
-- Add provider column to messages table
ALTER TABLE messages ADD COLUMN provider TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Remove provider column from messages table
ALTER TABLE messages DROP COLUMN provider;
-- +goose StatementEnd