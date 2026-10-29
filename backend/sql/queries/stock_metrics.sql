-- name: ListCurrentStockProductSummariesBySellerAccountID :many
WITH latest_stock_per_warehouse AS (
    SELECT DISTINCT ON (st.product_external_id, st.warehouse_external_id)
        st.product_external_id,
        st.warehouse_external_id,
        st.quantity_total,
        st.quantity_reserved,
        st.quantity_available,
        st.snapshot_at,
        st.id
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
    l.product_external_id::bigint AS ozon_product_id,
    p.offer_id,
    p.sku,
    p.name AS product_name,
    COUNT(*)::integer AS warehouse_count,
    COALESCE(SUM(l.quantity_total), 0)::integer AS total_stock,
    COALESCE(SUM(l.quantity_reserved), 0)::integer AS reserved_stock,
    COALESCE(SUM(l.quantity_available), 0)::integer AS available_stock,
    MAX(l.snapshot_at)::timestamptz AS snapshot_at
FROM latest_stock_per_warehouse l
LEFT JOIN products p
    ON p.seller_account_id = $1
   AND p.ozon_product_id = l.product_external_id::bigint
GROUP BY
    l.product_external_id::bigint,
    p.offer_id,
    p.sku,
    p.name
ORDER BY available_stock DESC, l.product_external_id::bigint ASC;

-- name: ListCurrentStockWarehouseRowsBySellerAccountID :many
WITH latest_stock_per_warehouse AS (
    SELECT DISTINCT ON (st.product_external_id, st.warehouse_external_id)
        st.product_external_id,
        st.warehouse_external_id,
        st.quantity_total,
        st.quantity_reserved,
        st.quantity_available,
        st.snapshot_at,
        st.id
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
    l.product_external_id::bigint AS ozon_product_id,
    l.warehouse_external_id,
    COALESCE(l.quantity_total, 0)::integer AS total_stock,
    COALESCE(l.quantity_reserved, 0)::integer AS reserved_stock,
    COALESCE(l.quantity_available, 0)::integer AS available_stock,
    l.snapshot_at
FROM latest_stock_per_warehouse l
ORDER BY l.product_external_id::bigint ASC, l.warehouse_external_id ASC;
