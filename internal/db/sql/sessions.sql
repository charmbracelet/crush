-- name: CreateSession :one
INSERT INTO sessions (
    id,
    parent_session_id,
    title,
    workspace_cwd,
    collaboration_mode,
    kind,
    handoff_source_session_id,
    handoff_goal,
    handoff_draft_prompt,
    handoff_relevant_files,
    message_count,
    prompt_tokens,
    completion_tokens,
    cost,
    summary_message_id,
    updated_at,
    created_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    null,
    strftime('%s', 'now'),
    strftime('%s', 'now')
) RETURNING *;

-- name: GetSessionByID :one
SELECT *
FROM sessions
WHERE id = ? LIMIT 1;

-- name: GetLastSession :one
SELECT *
FROM sessions
WHERE parent_session_id IS NULL
ORDER BY updated_at DESC
LIMIT 1;

-- name: ListSessions :many
SELECT *
FROM sessions
WHERE parent_session_id is NULL
ORDER BY updated_at DESC;

-- name: UpdateSession :one
UPDATE sessions
SET
    title = ?,
    workspace_cwd = ?,
    collaboration_mode = ?,
    kind = ?,
    handoff_source_session_id = ?,
    handoff_goal = ?,
    handoff_draft_prompt = ?,
    handoff_relevant_files = ?,
    prompt_tokens = ?,
    completion_tokens = ?,
    last_prompt_tokens = ?,
    last_completion_tokens = ?,
    summary_message_id = ?,
    cost = ?,
    todos = ?
WHERE id = ?
RETURNING *;

-- name: UpdateSessionCollaborationMode :one
UPDATE sessions
SET collaboration_mode = ?
WHERE id = ?
RETURNING *;

-- name: UpdateSessionTitleAndUsage :one
UPDATE sessions
SET
    title = ?,
    prompt_tokens = prompt_tokens + ?,
    completion_tokens = completion_tokens + ?,
    cost = cost + ?,
    updated_at = strftime('%s', 'now')
WHERE id = ?
RETURNING *;


-- name: RenameSession :exec
UPDATE sessions
SET
    title = ?
WHERE id = ?;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = ?;
