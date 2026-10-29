-- name: UpsertSale :one
INSERT INTO sales (
    seller_account_id,
    ozon_sale_id,
    ozon_order_id,
    posting_number,
    quantity,
    amount,
    currency_code,
    sale_date,
    raw_attributes,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, NOW()
)
ON CONFLICT (seller_account_id, ozon_sale_id)
DO UPDATE SET
    ozon_order_id = EXCLUDED.ozon_order_id,
    posting_number = EXCLUDED.posting_number,
    quantity = EXCLUDED.quantity,
    amount = EXCLUDED.amount,
    currency_code = EXCLUDED.currency_code,
    sale_date = EXCLUDED.sale_date,
    raw_attributes = EXCLUDED.raw_attributes,
    updated_at = NOW()
RETURNING *;

-- name: ListSalesBySellerAccountID :many
SELECT *
FROM sales
WHERE seller_account_id = $1
ORDER BY id ASC;

-- name: CountSalesBySellerAccountID :one
SELECT COUNT(*)
FROM sales
WHERE seller_account_id = $1;