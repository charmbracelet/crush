-- name: ListSkillsDisabled :many
SELECT name FROM disabled_skills ORDER BY name;

-- name: InsertSkillsDisabled :exec
INSERT OR IGNORE INTO disabled_skills (name) VALUES (?);

-- name: DeleteSkillsDisabled :exec
DELETE FROM disabled_skills WHERE name = ?;

-- name: ListSkillsEnabled :many
SELECT name FROM enabled_skills ORDER BY name;

-- name: InsertSkillsEnabled :exec
INSERT OR IGNORE INTO enabled_skills (name) VALUES (?);

-- name: DeleteSkillsEnabled :exec
DELETE FROM enabled_skills WHERE name = ?;
