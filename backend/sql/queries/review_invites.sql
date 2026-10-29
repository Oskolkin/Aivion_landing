-- name: CreateReviewInvite :one
INSERT INTO review_invites (
    token_hash,
    seller_account_id,
    user_id,
    expires_at,
    label
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetActiveReviewInviteByTokenHash :one
SELECT * FROM review_invites
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW()
LIMIT 1;

-- name: GetReviewInviteByID :one
SELECT * FROM review_invites
WHERE id = $1
LIMIT 1;

-- name: RevokeReviewInvite :one
UPDATE review_invites
SET revoked_at = NOW()
WHERE id = $1
  AND revoked_at IS NULL
RETURNING *;

-- name: ListActiveReviewInvites :many
SELECT * FROM review_invites
WHERE revoked_at IS NULL
  AND expires_at > NOW()
ORDER BY created_at DESC;
