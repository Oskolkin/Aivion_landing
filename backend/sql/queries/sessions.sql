-- name: CreateSession :one
INSERT INTO sessions (
    user_id,
    token_hash,
    expires_at,
    read_only,
    session_kind,
    review_invite_id
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions
WHERE token_hash = $1
LIMIT 1;

-- name: GetActiveSessionByTokenHash :one
SELECT * FROM sessions
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW()
LIMIT 1;

-- name: RevokeSessionByTokenHash :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE token_hash = $1
  AND revoked_at IS NULL;

-- name: RevokeSessionsByReviewInviteID :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE review_invite_id = $1
  AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at < NOW();
