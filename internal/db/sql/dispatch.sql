-- name: CreateDispatchMessage :one
INSERT INTO dispatch_messages (
    id,
    from_agent,
    to_agent,
    session_id,
    parent_message_id,
    task,
    context,
    status,
    priority,
    created_at,
    updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, 'pending', ?, strftime('%s', 'now'), strftime('%s', 'now')
) RETURNING *;

-- name: GetDispatchMessage :one
SELECT * FROM dispatch_messages WHERE id = ? LIMIT 1;

-- name: ListDispatchMessages :many
SELECT * FROM dispatch_messages
WHERE (? = '' OR from_agent = ?)
  AND (? = '' OR to_agent = ?)
  AND (? = '' OR status = ?)
ORDER BY priority DESC, created_at ASC;

-- name: PollDispatchMessages :many
SELECT * FROM dispatch_messages
WHERE to_agent = ?
  AND status = 'pending'
ORDER BY priority DESC, created_at ASC
LIMIT ?;

-- name: ClaimDispatchMessage :one
UPDATE dispatch_messages
SET status = 'in_progress', updated_at = strftime('%s', 'now')
WHERE id = ? AND status = 'pending'
RETURNING *;

-- name: CompleteDispatchMessage :one
UPDATE dispatch_messages
SET status = 'completed',
    result = ?,
    completed_at = strftime('%s', 'now'),
    updated_at = strftime('%s', 'now')
WHERE id = ?
RETURNING *;

-- name: FailDispatchMessage :one
UPDATE dispatch_messages
SET status = 'failed',
    error = ?,
    completed_at = strftime('%s', 'now'),
    updated_at = strftime('%s', 'now')
WHERE id = ?
RETURNING *;

-- name: DeleteDispatchMessage :exec
DELETE FROM dispatch_messages WHERE id = ?;

-- name: ListPendingDispatchMessages :many
SELECT * FROM dispatch_messages
WHERE status = 'pending'
ORDER BY created_at ASC;

-- name: ListStaleDispatchMessages :many
SELECT * FROM dispatch_messages
WHERE status = 'in_progress'
  AND updated_at < ?
ORDER BY updated_at ASC;

-- name: ResetDispatchMessage :one
UPDATE dispatch_messages
SET status = 'pending',
    updated_at = strftime('%s', 'now')
WHERE id = ?
RETURNING *;

-- name: CreateAgent :one
INSERT INTO agents (
    name,
    description,
    capabilities,
    system_prompt,
    cli_command,
    model_requirements,
    created_at,
    updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, strftime('%s', 'now'), strftime('%s', 'now')
) RETURNING *;

-- name: GetAgent :one
SELECT * FROM agents WHERE name = ? LIMIT 1;

-- name: ListAgents :many
SELECT * FROM agents WHERE enabled = 1 ORDER BY name;

-- name: ListAllAgents :many
SELECT * FROM agents ORDER BY name;

-- name: UpdateAgent :one
UPDATE agents
SET description = ?,
    capabilities = ?,
    system_prompt = ?,
    cli_command = ?,
    model_requirements = ?,
    updated_at = strftime('%s', 'now')
WHERE name = ?
RETURNING *;

-- name: SetAgentEnabled :exec
UPDATE agents SET enabled = ?, updated_at = strftime('%s', 'now') WHERE name = ?;

-- name: DeleteAgent :exec
DELETE FROM agents WHERE name = ?;
