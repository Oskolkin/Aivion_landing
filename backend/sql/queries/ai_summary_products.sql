-- name: ListDescriptionCategoryRollupForAISummary :many
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
),
latest_product_stock AS (
    SELECT
        product_external_id::bigint AS ozon_product_id,
        COALESCE(SUM(quantity_available), 0)::integer AS stock_available
    FROM latest_stock_per_warehouse
    GROUP BY product_external_id::bigint
)
SELECT
    p.description_category_id AS description_category_id,
    COUNT(*)::bigint AS active_product_count,
    COUNT(*) FILTER (
        WHERE COALESCE(dsm.stock_available, ls.stock_available, 0) > 0
    )::bigint AS products_with_stock_count,
    COALESCE(SUM(dsm.revenue), 0)::numeric AS revenue_sum,
    COALESCE(SUM(dsm.orders_count), 0)::bigint AS orders_sum,
    COALESCE(SUM(dsm.units_sold), 0)::bigint AS units_sum
FROM products p
LEFT JOIN daily_sku_metrics dsm ON dsm.seller_account_id = p.seller_account_id
    AND dsm.ozon_product_id = p.ozon_product_id
    AND dsm.metric_date = $2
LEFT JOIN latest_product_stock ls ON ls.ozon_product_id = p.ozon_product_id
WHERE p.seller_account_id = $1
  AND p.is_archived = false
  AND p.description_category_id IS NOT NULL
GROUP BY p.description_category_id
HAVING COUNT(*) FILTER (
    WHERE COALESCE(dsm.stock_available, ls.stock_available, 0) > 0
) > 0
ORDER BY revenue_sum DESC, description_category_id ASC;

-- name: ListProductSKUMetricsForCategoryAndDate :many
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
),
latest_product_stock AS (
    SELECT
        product_external_id::bigint AS ozon_product_id,
        COALESCE(SUM(quantity_available), 0)::integer AS stock_available
    FROM latest_stock_per_warehouse
    GROUP BY product_external_id::bigint
)
SELECT
    p.ozon_product_id,
    p.sku,
    p.offer_id,
    p.name AS product_name,
    COALESCE(dsm.revenue, 0)::numeric AS revenue,
    COALESCE(dsm.orders_count, 0)::integer AS orders_count,
    COALESCE(dsm.units_sold, 0)::integer AS units_sold,
    COALESCE(dsm.stock_available, ls.stock_available, 0)::integer AS stock_available,
    dsm.days_of_cover
FROM products p
LEFT JOIN daily_sku_metrics dsm ON dsm.seller_account_id = p.seller_account_id
    AND dsm.ozon_product_id = p.ozon_product_id
    AND dsm.metric_date = $3
LEFT JOIN latest_product_stock ls ON ls.ozon_product_id = p.ozon_product_id
WHERE p.seller_account_id = $1
  AND p.is_archived = false
  AND p.description_category_id = $2;

-- name: ListProductSkuCategoryBySellerForAISummary :many
SELECT DISTINCT
    p.sku AS sku,
    p.description_category_id AS description_category_id
FROM products p
WHERE p.seller_account_id = $1
  AND p.is_archived = false
  AND p.sku IS NOT NULL
  AND p.description_category_id IS NOT NULL;

-- name: ListLowCoverCategoryIdsForAISummary :many
SELECT DISTINCT p.description_category_id AS description_category_id
FROM products p
INNER JOIN daily_sku_metrics dsm ON dsm.seller_account_id = p.seller_account_id
    AND dsm.ozon_product_id = p.ozon_product_id
    AND dsm.metric_date = $2
WHERE p.seller_account_id = $1
  AND p.is_archived = false
  AND p.description_category_id IS NOT NULL
  AND dsm.days_of_cover IS NOT NULL
  AND dsm.days_of_cover < 7;
