-- name: GetSyncCursorBySellerAccountDomainAndType :one
SELECT *
FROM sync_cursors
WHERE seller_account_id = $1
  AND domain = $2
  AND cursor_type = $3
LIMIT 1;

-- name: ListSyncCursorsBySellerAccountID :many
SELECT *
FROM sync_cursors
WHERE seller_account_id = $1
ORDER BY domain, cursor_type;

-- name: UpsertSyncCursor :one
INSERT INTO sync_cursors (
    seller_account_id,
    domain,
    cursor_type,
    cursor_value,
    updated_at
) VALUES (
    $1, $2, $3, $4, NOW()
)
ON CONFLICT (seller_account_id, domain, cursor_type)
DO UPDATE SET
    cursor_value = EXCLUDED.cursor_value,
    updated_at = NOW()
RETURNING *;