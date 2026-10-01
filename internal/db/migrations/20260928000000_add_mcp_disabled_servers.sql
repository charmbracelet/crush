-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS mcp_disabled_servers (
    name TEXT PRIMARY KEY
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS mcp_disabled_servers;
-- +goose StatementEnd
