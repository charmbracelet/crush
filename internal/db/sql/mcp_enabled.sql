-- name: ListMCPEnabledServers :many
SELECT name FROM mcp_enabled_servers ORDER BY name;

-- name: InsertMCPEnabledServer :exec
INSERT OR IGNORE INTO mcp_enabled_servers (name) VALUES (?);

-- name: DeleteMCPEnabledServer :exec
DELETE FROM mcp_enabled_servers WHERE name = ?;
