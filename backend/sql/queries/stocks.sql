-- name: UpsertStock :one
INSERT INTO stocks (
    seller_account_id,
    product_external_id,
    warehouse_external_id,
    quantity_total,
    quantity_reserved,
    quantity_available,
    snapshot_at,
    raw_attributes,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, NOW()
)
ON CONFLICT (seller_account_id, product_external_id, warehouse_external_id)
DO UPDATE SET
    quantity_total = EXCLUDED.quantity_total,
    quantity_reserved = EXCLUDED.quantity_reserved,
    quantity_available = EXCLUDED.quantity_available,
    snapshot_at = EXCLUDED.snapshot_at,
    raw_attributes = EXCLUDED.raw_attributes,
    updated_at = NOW()
RETURNING *;

-- name: ListStocksBySellerAccountID :many
SELECT *
FROM stocks
WHERE seller_account_id = $1
ORDER BY id ASC;

-- name: CountStocksBySellerAccountID :one
SELECT COUNT(*)
FROM stocks
WHERE seller_account_id = $1;