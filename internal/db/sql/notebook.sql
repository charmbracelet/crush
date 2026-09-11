-- name: CreateNotebookEntry :one
INSERT INTO notebook_entries (
    id,
    session_id,
    turn_number,
    event_number,
    event_type,
    title,
    entry_text,
    entry_text_full,
    token_count,
    compression_level,
    succeeded,
    error_headline,
    created_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: CreateNotebookTag :exec
INSERT OR IGNORE INTO notebook_tags (entry_id, tag)
VALUES (?, ?);

-- name: GetNotebookEntries :many
SELECT *
FROM notebook_entries
WHERE session_id = ?
ORDER BY turn_number ASC, event_number ASC;

-- name: GetNotebookEntriesByTurn :many
SELECT *
FROM notebook_entries
WHERE session_id = ? AND turn_number = ?
ORDER BY event_number ASC;

-- name: GetNotebookEntriesByEventType :many
SELECT *
FROM notebook_entries
WHERE session_id = ? AND event_type = ?
ORDER BY turn_number ASC, event_number ASC;

-- name: SearchNotebookByTag :many
SELECT DISTINCT e.*
FROM notebook_entries e
JOIN notebook_tags t ON t.entry_id = e.id
WHERE e.session_id = ? AND t.tag = ?
ORDER BY e.turn_number ASC, e.event_number ASC;

-- name: SearchNotebookByText :many
SELECT *
FROM notebook_entries
WHERE session_id = ? AND entry_text LIKE sqlc.arg(search_term)
ORDER BY turn_number ASC, event_number ASC;

-- name: GetNotebookTagsByEntry :many
SELECT tag
FROM notebook_tags
WHERE entry_id = ?;

-- name: GetNotebookTokenCount :one
SELECT CAST(COALESCE(SUM(token_count), 0) AS INTEGER) as total_tokens
FROM notebook_entries
WHERE session_id = ?;

-- name: GetNotebookEntryCount :one
SELECT COUNT(*) AS entry_count
FROM notebook_entries
WHERE session_id = ?;

-- name: UpdateNotebookCompression :exec
UPDATE notebook_entries
SET
    entry_text = ?,
    token_count = ?,
    compression_level = ?
WHERE id = ?;

-- name: DeleteNotebookEntriesBySession :exec
DELETE FROM notebook_entries
WHERE session_id = ?;

-- name: GetOldestNotebookEntries :many
SELECT *
FROM notebook_entries
WHERE session_id = ? AND compression_level = ?
ORDER BY turn_number ASC, event_number ASC
LIMIT ?;
