-- name: CreateUser :one
INSERT INTO users (
    email,
    password_hash,
    status
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1
LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1
LIMIT 1;

-- name: UpdateUserPasswordHashByUserID :exec
UPDATE users
SET password_hash = $1,
    updated_at = NOW()
WHERE id = $2;