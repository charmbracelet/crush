-- name: CreateLedgerEntry :execrows
INSERT OR IGNORE INTO compaction_ledger (
    id, session_id, seq, kind, text, relevance, sources, retires, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListLedgerEntries :many
SELECT * FROM compaction_ledger WHERE session_id = ? ORDER BY seq ASC;

-- name: CountLedgerEntries :one
SELECT COUNT(*) FROM compaction_ledger WHERE session_id = ?;

-- name: MaxLedgerSeq :one
SELECT CAST(COALESCE(MAX(seq), 0) AS INTEGER) FROM compaction_ledger WHERE session_id = ?;

-- name: DeleteLedgerEntriesForSession :execrows
DELETE FROM compaction_ledger WHERE session_id = ?;
