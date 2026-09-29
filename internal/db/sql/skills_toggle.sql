-- name: ListSkillsDisabled :many
SELECT name FROM skills_disabled_servers ORDER BY name;

-- name: InsertSkillsDisabled :exec
INSERT OR IGNORE INTO skills_disabled_servers (name) VALUES (?);

-- name: DeleteSkillsDisabled :exec
DELETE FROM skills_disabled_servers WHERE name = ?;

-- name: ListSkillsEnabled :many
SELECT name FROM skills_enabled_servers ORDER BY name;

-- name: InsertSkillsEnabled :exec
INSERT OR IGNORE INTO skills_enabled_servers (name) VALUES (?);

-- name: DeleteSkillsEnabled :exec
DELETE FROM skills_enabled_servers WHERE name = ?;
