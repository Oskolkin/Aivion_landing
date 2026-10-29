-- name: UpsertOrder :one
INSERT INTO orders (
    seller_account_id,
    ozon_order_id,
    posting_number,
    status,
    created_at_source,
    processed_at_source,
    total_amount,
    currency_code,
    raw_attributes,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, NOW()
)
ON CONFLICT (seller_account_id, ozon_order_id)
DO UPDATE SET
    posting_number = EXCLUDED.posting_number,
    status = EXCLUDED.status,
    created_at_source = EXCLUDED.created_at_source,
    processed_at_source = EXCLUDED.processed_at_source,
    total_amount = EXCLUDED.total_amount,
    currency_code = EXCLUDED.currency_code,
    raw_attributes = EXCLUDED.raw_attributes,
    updated_at = NOW()
RETURNING *;

-- name: ListOrdersBySellerAccountID :many
SELECT *
FROM orders
WHERE seller_account_id = $1
ORDER BY id ASC;

-- name: CountOrdersBySellerAccountID :one
SELECT COUNT(*)
FROM orders
WHERE seller_account_id = $1;

-- name: CountOrdersByStatusForDateRange :many
SELECT
    COALESCE(NULLIF(LOWER(TRIM(o.status)), ''), 'unknown')::text AS status,
    COUNT(*)::bigint AS orders_count
FROM orders o
WHERE o.seller_account_id = $1
  AND o.created_at_source IS NOT NULL
  AND o.created_at_source::date >= $2::date
  AND o.created_at_source::date <= $3::date
GROUP BY 1
ORDER BY orders_count DESC;