-- name: RecordFileWrite :exec
INSERT INTO written_files (
    session_id,
    path,
    written_at
) VALUES (
    ?,
    ?,
    strftime('%s', 'now')
) ON CONFLICT(path, session_id) DO UPDATE SET
    written_at = excluded.written_at;

-- name: GetFileWrite :one
SELECT * FROM written_files
WHERE session_id = ? AND path = ? LIMIT 1;

-- name: ListSessionWrittenFiles :many
SELECT * FROM written_files
WHERE session_id = ?
ORDER BY written_at DESC;
