-- name: CreateSellerAccount :one
INSERT INTO seller_accounts (
    user_id,
    name,
    status
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: GetSellerAccountByUserID :one
SELECT * FROM seller_accounts
WHERE user_id = $1
LIMIT 1;

-- name: GetSellerAccountByID :one
SELECT * FROM seller_accounts
WHERE id = $1
LIMIT 1;