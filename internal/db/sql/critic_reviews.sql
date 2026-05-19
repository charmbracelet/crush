-- name: CreateCriticReview :one
INSERT INTO critic_reviews (
    id,
    session_id,
    message_id,
    verdict,
    confidence,
    concerns,
    summary,
    diff_snapshot,
    lsp_diagnostics,
    created_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now')
)
RETURNING id, session_id, message_id, verdict, confidence, concerns, summary, diff_snapshot, lsp_diagnostics, created_at
;

-- name: GetCriticReviewByMessageID :one
SELECT id, session_id, message_id, verdict, confidence, concerns, summary, diff_snapshot, lsp_diagnostics, created_at
FROM critic_reviews
WHERE message_id = ? LIMIT 1
;

-- name: ListCriticReviewsBySession :many
SELECT id, session_id, message_id, verdict, confidence, concerns, summary, diff_snapshot, lsp_diagnostics, created_at
FROM critic_reviews
WHERE session_id = ?
ORDER BY created_at DESC
;

-- name: PruneCriticReviews :execrows
DELETE FROM critic_reviews
WHERE created_at < ?
;
