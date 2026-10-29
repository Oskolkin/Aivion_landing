-- name: UpsertProductAnalyticsDaily :one
INSERT INTO product_analytics_daily (
    seller_account_id,
    metric_date,
    scope_type,
    scope_id,
    impressions,
    product_views,
    add_to_cart,
    orders_count,
    completed_orders,
    revenue,
    raw_payload_id,
    source_status,
    source_message,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW()
)
ON CONFLICT (seller_account_id, metric_date, scope_type, scope_id)
DO UPDATE SET
    impressions = EXCLUDED.impressions,
    product_views = EXCLUDED.product_views,
    add_to_cart = EXCLUDED.add_to_cart,
    orders_count = EXCLUDED.orders_count,
    completed_orders = EXCLUDED.completed_orders,
    revenue = EXCLUDED.revenue,
    raw_payload_id = EXCLUDED.raw_payload_id,
    source_status = EXCLUDED.source_status,
    source_message = EXCLUDED.source_message,
    updated_at = NOW()
RETURNING *;

-- name: GetProductAnalyticsDailyBySellerDateScope :one
SELECT *
FROM product_analytics_daily
WHERE seller_account_id = $1
  AND metric_date = $2
  AND scope_type = $3
  AND scope_id = $4;

-- name: ListProductAnalyticsDailyBySellerAndDateRange :many
SELECT *
FROM product_analytics_daily
WHERE seller_account_id = $1
  AND metric_date >= $2
  AND metric_date <= $3
ORDER BY metric_date ASC;
