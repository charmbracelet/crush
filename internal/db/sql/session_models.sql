-- name: UpsertSessionModel :one
INSERT INTO session_models (
    session_id,
    model_type,
    provider,
    model,
    selected_model_json,
    updated_at,
    created_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    strftime('%s', 'now'),
    strftime('%s', 'now')
)
ON CONFLICT(session_id, model_type) DO UPDATE SET
    provider = excluded.provider,
    model = excluded.model,
    selected_model_json = excluded.selected_model_json,
    updated_at = strftime('%s', 'now')
RETURNING *;

-- name: ListSessionModels :many
SELECT *
FROM session_models
WHERE session_id = ?
ORDER BY model_type;

-- name: DeleteSessionModels :exec
DELETE FROM session_models
WHERE session_id = ?;
