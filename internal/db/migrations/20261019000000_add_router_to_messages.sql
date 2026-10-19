-- +goose Up
-- +goose StatementBegin
-- Add router columns to messages table. Set on turns whose model was
-- picked by a classifier router.
ALTER TABLE messages ADD COLUMN router_name TEXT;
ALTER TABLE messages ADD COLUMN route_name TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE messages DROP COLUMN router_name;
ALTER TABLE messages DROP COLUMN route_name;
-- +goose StatementEnd
