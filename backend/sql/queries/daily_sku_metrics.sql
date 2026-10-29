-- name: DeleteDailySKUMetricsBySellerAndDateRange :exec
DELETE FROM daily_sku_metrics
WHERE seller_account_id = $1
  AND metric_date >= $2
  AND metric_date <= $3;

-- name: UpsertDailySKUMetric :one
INSERT INTO daily_sku_metrics (
    seller_account_id,
    metric_date,
    ozon_product_id,
    offer_id,
    sku,
    product_name,
    revenue,
    orders_count,
    units_sold,
    stock_available,
    days_of_cover,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW()
)
ON CONFLICT (seller_account_id, metric_date, ozon_product_id)
DO UPDATE SET
    offer_id = EXCLUDED.offer_id,
    sku = EXCLUDED.sku,
    product_name = EXCLUDED.product_name,
    revenue = EXCLUDED.revenue,
    orders_count = EXCLUDED.orders_count,
    units_sold = EXCLUDED.units_sold,
    stock_available = EXCLUDED.stock_available,
    days_of_cover = EXCLUDED.days_of_cover,
    updated_at = NOW()
RETURNING *;

-- name: ListDailySKUMetricsBySellerAndDateRange :many
SELECT *
FROM daily_sku_metrics
WHERE seller_account_id = $1
  AND metric_date >= $2
  AND metric_date <= $3
ORDER BY metric_date ASC, ozon_product_id ASC;

-- name: ListDailySKUMetricsBySellerProductsAndDateRange :many
SELECT *
FROM daily_sku_metrics
WHERE seller_account_id = $1
  AND metric_date >= $2
  AND metric_date <= $3
  AND ozon_product_id = ANY($4::bigint[])
ORDER BY metric_date ASC, ozon_product_id ASC;

-- name: GetDailySKUMetricSourceDateBoundsBySellerAccountID :one
WITH source_dates AS (
    SELECT s.sale_date::date AS metric_date
    FROM sales s
    WHERE s.seller_account_id = $1
      AND s.sale_date IS NOT NULL
      AND (
        (s.raw_attributes->>'product_id') ~ '^[0-9]+$'
        OR (s.raw_attributes->>'ozon_product_id') ~ '^[0-9]+$'
      )
)
SELECT
    MIN(metric_date)::date AS min_date,
    MAX(metric_date)::date AS max_date
FROM source_dates;

-- name: ListCurrentStockBySellerAccountAndProduct :many
WITH latest_stock_per_warehouse AS (
    SELECT DISTINCT ON (st.product_external_id, st.warehouse_external_id)
        st.product_external_id,
        st.warehouse_external_id,
        st.quantity_available,
        st.snapshot_at
    FROM stocks st
    WHERE st.seller_account_id = $1
      AND st.product_external_id ~ '^[0-9]+$'
    ORDER BY
        st.product_external_id,
        st.warehouse_external_id,
        st.snapshot_at DESC NULLS LAST,
        st.id DESC
)
SELECT
    product_external_id::bigint AS ozon_product_id,
    COALESCE(SUM(quantity_available), 0)::integer AS stock_available
FROM latest_stock_per_warehouse
GROUP BY product_external_id
ORDER BY product_external_id::bigint;

-- name: ListDailySKUSourcesBySellerAndDateRange :many
WITH sales_by_sku_day AS (
    SELECT
        s.sale_date::date AS metric_date,
        COALESCE(
            NULLIF(s.raw_attributes->>'product_id', ''),
            NULLIF(s.raw_attributes->>'ozon_product_id', '')
        )::bigint AS ozon_product_id,
        COALESCE(SUM(s.amount), 0)::numeric(18,2) AS revenue,
        COUNT(*)::integer AS orders_count,
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
      AND (
        (s.raw_attributes->>'product_id') ~ '^[0-9]+$'
        OR (s.raw_attributes->>'ozon_product_id') ~ '^[0-9]+$'
      )
    GROUP BY
        s.sale_date::date,
        COALESCE(
            NULLIF(s.raw_attributes->>'product_id', ''),
            NULLIF(s.raw_attributes->>'ozon_product_id', '')
        )::bigint
),
stock_rows_for_end_date AS (
    SELECT
        $3::date AS metric_date,
        p.ozon_product_id,
        0::numeric(18,2) AS revenue,
        0::integer AS orders_count,
        0::integer AS units_sold
    FROM products p
    WHERE p.seller_account_id = $1
),
all_rows AS (
    SELECT metric_date, ozon_product_id, revenue, orders_count, units_sold FROM sales_by_sku_day
    UNION
    SELECT metric_date, ozon_product_id, revenue, orders_count, units_sold FROM stock_rows_for_end_date
)
SELECT
    r.metric_date,
    r.ozon_product_id,
    p.offer_id,
    p.sku,
    p.name AS product_name,
    COALESCE(SUM(r.revenue), 0)::numeric(18,2) AS revenue,
    COALESCE(SUM(r.orders_count), 0)::integer AS orders_count,
    COALESCE(SUM(r.units_sold), 0)::integer AS units_sold
FROM all_rows r
LEFT JOIN products p
    ON p.seller_account_id = $1
   AND p.ozon_product_id = r.ozon_product_id
GROUP BY
    r.metric_date,
    r.ozon_product_id,
    p.offer_id,
    p.sku,
    p.name
ORDER BY r.metric_date ASC, r.ozon_product_id ASC;
