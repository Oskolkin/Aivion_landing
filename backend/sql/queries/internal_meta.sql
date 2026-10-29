-- name: GetInternalMetaValue :one
SELECT value
FROM internal_meta
WHERE key = $1;

-- name: UpsertInternalMetaValue :exec
INSERT INTO internal_meta (key, value, updated_at)
VALUES ($1, $2, NOW())
ON CONFLICT (key) DO UPDATE SET
    value = EXCLUDED.value,
    updated_at = NOW();
