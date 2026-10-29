-- name: DeleteDailyAccountMetricsBySellerAndDateRange :exec
DELETE FROM daily_account_metrics
WHERE seller_account_id = $1
  AND metric_date >= $2
  AND metric_date <= $3;

-- name: UpsertDailyAccountMetric :one
INSERT INTO daily_account_metrics (
    seller_account_id,
    metric_date,
    revenue,
    orders_count,
    returns_count,
    cancel_count,
    units_sold,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, NOW()
)
ON CONFLICT (seller_account_id, metric_date)
DO UPDATE SET
    revenue = EXCLUDED.revenue,
    orders_count = EXCLUDED.orders_count,
    returns_count = EXCLUDED.returns_count,
    cancel_count = EXCLUDED.cancel_count,
    units_sold = EXCLUDED.units_sold,
    updated_at = NOW()
RETURNING *;

-- name: ListDailyAccountMetricsBySellerAndDateRange :many
SELECT *
FROM daily_account_metrics
WHERE seller_account_id = $1
  AND metric_date >= $2
  AND metric_date <= $3
ORDER BY metric_date ASC;

-- name: GetLatestAvailableDashboardMetricDateBySellerAccountID :one
-- Prefer account-level anchor (same as MVP seed validation); fall back to SKU max only when account metrics are absent.
SELECT COALESCE(
    (SELECT MAX(dam.metric_date) FROM daily_account_metrics dam WHERE dam.seller_account_id = $1),
    (SELECT MAX(dsm.metric_date) FROM daily_sku_metrics dsm WHERE dsm.seller_account_id = $1),
    '1970-01-01'::date
)::date AS latest_date;

-- name: GetDailyAccountMetricSourceDateBoundsBySellerAccountID :one
WITH source_dates AS (
    SELECT sale_date::date AS metric_date
    FROM sales s
    WHERE s.seller_account_id = $1
      AND s.sale_date IS NOT NULL

    UNION ALL

    SELECT created_at_source::date AS metric_date
    FROM orders o
    WHERE o.seller_account_id = $1
      AND o.created_at_source IS NOT NULL
)
SELECT
    MIN(metric_date)::date AS min_date,
    MAX(metric_date)::date AS max_date
FROM source_dates;

-- name: ListDailyAccountMetricSourcesBySellerAndDateRange :many
WITH sales_by_day AS (
    SELECT
        s.sale_date::date AS metric_date,
        COALESCE(SUM(s.amount), 0)::numeric(18,2) AS revenue,
        COALESCE(SUM(s.quantity) FILTER (
            WHERE COALESCE(s.quantity, 0) > 0
              AND COALESCE(s.amount, 0) > 0
              AND LOWER(COALESCE(s.raw_attributes->>'operation_type', '')) NOT LIKE '%return%'
              AND LOWER(COALESCE(s.raw_attributes->>'operation_type', '')) NOT LIKE '%cancel%'
        ), 0)::integer AS units_sold
    FROM sales s
    WHERE s.seller_account_id = $1
      AND s.sale_date IS NOT NULL
      AND s.sale_date::date >= $2::date
      AND s.sale_date::date <= $3::date
    GROUP BY s.sale_date::date
),
orders_by_day AS (
    SELECT
        o.created_at_source::date AS metric_date,
        COUNT(*)::integer AS orders_count,
        COUNT(*) FILTER (WHERE LOWER(COALESCE(o.status, '')) = 'returned')::integer AS returns_count,
        COUNT(*) FILTER (WHERE LOWER(COALESCE(o.status, '')) = 'cancelled')::integer AS cancel_count
    FROM orders o
    WHERE o.seller_account_id = $1
      AND o.created_at_source IS NOT NULL
      AND o.created_at_source::date >= $2::date
      AND o.created_at_source::date <= $3::date
    GROUP BY o.created_at_source::date
),
days AS (
    SELECT metric_date FROM sales_by_day
    UNION
    SELECT metric_date FROM orders_by_day
)
SELECT
    d.metric_date,
    COALESCE(s.revenue, 0)::numeric(18,2) AS revenue,
    COALESCE(o.orders_count, 0)::integer AS orders_count,
    COALESCE(o.returns_count, 0)::integer AS returns_count,
    COALESCE(o.cancel_count, 0)::integer AS cancel_count,
    COALESCE(s.units_sold, 0)::integer AS units_sold
FROM days d
LEFT JOIN sales_by_day s ON s.metric_date = d.metric_date
LEFT JOIN orders_by_day o ON o.metric_date = d.metric_date
ORDER BY d.metric_date ASC;
