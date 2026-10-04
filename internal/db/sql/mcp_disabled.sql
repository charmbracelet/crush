-- name: ListMCPDisabledServers :many
SELECT name FROM mcp_disabled_servers ORDER BY name;

-- name: InsertMCPDisabledServer :exec
INSERT OR IGNORE INTO mcp_disabled_servers (name) VALUES (?);

-- name: DeleteMCPDisabledServer :exec
DELETE FROM mcp_disabled_servers WHERE name = ?;
