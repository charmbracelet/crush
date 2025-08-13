-- name: CreateTodo :one
INSERT INTO todos (
    id,
    session_id,
    content,
    status,
    created_at,
    updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: GetTodo :one
SELECT * FROM todos
WHERE id = ?;

-- name: ListTodosBySession :many
SELECT * FROM todos
WHERE session_id = ?
ORDER BY created_at ASC;

-- name: ListTodosBySessionAndStatus :many
SELECT * FROM todos
WHERE session_id = ? AND status = ?
ORDER BY created_at ASC;

-- name: UpdateTodo :one
UPDATE todos
SET content = ?, status = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: UpdateTodoStatus :one
UPDATE todos
SET status = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteTodo :exec
DELETE FROM todos
WHERE id = ?;

-- name: DeleteTodosBySession :exec
DELETE FROM todos
WHERE session_id = ?;

-- name: CountTodosBySessionAndStatus :one
SELECT COUNT(*) FROM todos
WHERE session_id = ? AND status = ?;