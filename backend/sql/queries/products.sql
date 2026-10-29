-- name: UpsertProduct :one
INSERT INTO products (
    seller_account_id,
    ozon_product_id,
    offer_id,
    sku,
    name,
    status,
    reference_price,
    old_price,
    ozon_min_price,
    description_category_id,
    is_archived,
    raw_attributes,
    primary_image_url,
    image_urls,
    source_updated_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW()
)
ON CONFLICT (seller_account_id, ozon_product_id)
DO UPDATE SET
    offer_id = EXCLUDED.offer_id,
    sku = EXCLUDED.sku,
    name = EXCLUDED.name,
    status = EXCLUDED.status,
    reference_price = EXCLUDED.reference_price,
    old_price = EXCLUDED.old_price,
    ozon_min_price = EXCLUDED.ozon_min_price,
    description_category_id = EXCLUDED.description_category_id,
    is_archived = EXCLUDED.is_archived,
    raw_attributes = EXCLUDED.raw_attributes,
    primary_image_url = EXCLUDED.primary_image_url,
    image_urls = EXCLUDED.image_urls,
    source_updated_at = EXCLUDED.source_updated_at,
    updated_at = NOW()
RETURNING *;

-- name: ListProductsBySellerAccountID :many
SELECT *
FROM products
WHERE seller_account_id = $1
ORDER BY id ASC;

-- name: CountProductsBySellerAccountID :one
SELECT COUNT(*)
FROM products
WHERE seller_account_id = $1;

-- name: GetProductBySellerAndOzonProductID :one
SELECT *
FROM products
WHERE seller_account_id = $1
  AND ozon_product_id = $2;

-- name: ListProductCardsByOzonProductIDs :many
SELECT
    ozon_product_id,
    sku,
    offer_id,
    name,
    primary_image_url
FROM products
WHERE seller_account_id = $1
  AND ozon_product_id = ANY(sqlc.arg(ozon_product_ids)::bigint[]);

-- name: ListProductPricingBySellerAccountID :many
SELECT
    ozon_product_id,
    sku,
    old_price,
    reference_price
FROM products
WHERE seller_account_id = $1;

-- name: ProductExistsBySellerAndSKU :one
SELECT EXISTS(
    SELECT 1
    FROM products
    WHERE seller_account_id = $1
      AND sku = $2
)::boolean AS exists;

-- name: ProductExistsBySellerAndOfferID :one
SELECT EXISTS(
    SELECT 1
    FROM products
    WHERE seller_account_id = $1
      AND offer_id = $2
)::boolean AS exists;

-- name: GetLatestSKUMetricDateBySellerAccountID :one
SELECT MAX(metric_date)::date AS latest_date
FROM daily_sku_metrics
WHERE seller_account_id = $1;

-- name: CountProductsCatalogBySeller :one
WITH latest AS (
    SELECT MAX(metric_date) AS metric_date
    FROM daily_sku_metrics
    WHERE seller_account_id = sqlc.arg(seller_account_id)
)
SELECT COUNT(*)::bigint
FROM products p
LEFT JOIN latest l ON true
LEFT JOIN daily_sku_metrics dsm
    ON dsm.seller_account_id = p.seller_account_id
   AND dsm.ozon_product_id = p.ozon_product_id
   AND dsm.metric_date = l.metric_date
WHERE p.seller_account_id = sqlc.arg(seller_account_id)
  AND (
    sqlc.narg(search)::text IS NULL
    OR p.name ILIKE '%' || sqlc.narg(search)::text || '%'
    OR p.offer_id ILIKE '%' || sqlc.narg(search)::text || '%'
    OR p.sku::text LIKE '%' || sqlc.narg(search)::text || '%'
  )
  AND (
    sqlc.narg(has_sales_last_day)::boolean IS NULL
    OR (
      sqlc.narg(has_sales_last_day)::boolean = TRUE
      AND COALESCE(dsm.orders_count, 0) > 0
    )
    OR (
      sqlc.narg(has_sales_last_day)::boolean = FALSE
      AND COALESCE(dsm.orders_count, 0) = 0
    )
  );

-- name: ListProductsCatalogPage :many
WITH latest AS (
    SELECT MAX(metric_date) AS metric_date
    FROM daily_sku_metrics
    WHERE seller_account_id = sqlc.arg(seller_account_id)
)
SELECT
    p.ozon_product_id,
    p.offer_id,
    p.sku,
    p.name,
    p.reference_price,
    p.old_price,
    p.ozon_min_price,
    p.primary_image_url,
    l.metric_date AS last_metric_date,
    dsm.orders_count AS last_day_orders_count,
    dsm.revenue AS last_day_revenue,
    dsm.units_sold AS last_day_units_sold,
    dsm.stock_available AS last_day_stock_available,
    dsm.days_of_cover AS last_day_days_of_cover
FROM products p
LEFT JOIN latest l ON true
LEFT JOIN daily_sku_metrics dsm
    ON dsm.seller_account_id = p.seller_account_id
   AND dsm.ozon_product_id = p.ozon_product_id
   AND dsm.metric_date = l.metric_date
WHERE p.seller_account_id = sqlc.arg(seller_account_id)
  AND (
    sqlc.narg(search)::text IS NULL
    OR p.name ILIKE '%' || sqlc.narg(search)::text || '%'
    OR p.offer_id ILIKE '%' || sqlc.narg(search)::text || '%'
    OR p.sku::text LIKE '%' || sqlc.narg(search)::text || '%'
  )
  AND (
    sqlc.narg(has_sales_last_day)::boolean IS NULL
    OR (
      sqlc.narg(has_sales_last_day)::boolean = TRUE
      AND COALESCE(dsm.orders_count, 0) > 0
    )
    OR (
      sqlc.narg(has_sales_last_day)::boolean = FALSE
      AND COALESCE(dsm.orders_count, 0) = 0
    )
  )
ORDER BY
    CASE WHEN sqlc.arg(sort_by)::text = 'orders_last_day' THEN COALESCE(dsm.orders_count, 0) END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort_by)::text = 'revenue_last_day' THEN COALESCE(dsm.revenue, 0) END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort_by)::text = 'current_price_desc' THEN COALESCE(p.reference_price, 0) END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort_by)::text = 'current_price_asc' THEN COALESCE(p.reference_price, 0) END ASC NULLS LAST,
    CASE WHEN sqlc.arg(sort_by)::text = 'stock_available_asc' THEN COALESCE(dsm.stock_available, 0) END ASC NULLS LAST,
    CASE WHEN sqlc.arg(sort_by)::text = 'product_name_asc' THEN p.name END ASC NULLS LAST,
    p.ozon_product_id ASC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);