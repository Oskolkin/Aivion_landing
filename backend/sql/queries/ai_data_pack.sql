-- name: ListOzonProductInfoSnapshotsBySellerAndDate :many
SELECT *
FROM ozon_product_info_snapshots
WHERE seller_account_id = $1
  AND snapshot_date = $2
ORDER BY COALESCE(sku, 0) ASC, COALESCE(product_id, 0) ASC;

-- name: ListOzonRelatedSKUsBySeller :many
SELECT *
FROM ozon_related_skus
WHERE seller_account_id = $1
ORDER BY requested_sku ASC, related_sku ASC;

-- name: ListDescriptionCategoriesAll :many
SELECT *
FROM description_categories
ORDER BY description_category_id ASC;

-- name: CountDailySKUMetricsBySellerAndDateRange :one
SELECT COUNT(*)::bigint AS count
FROM daily_sku_metrics
WHERE seller_account_id = $1
  AND metric_date >= $2
  AND metric_date <= $3;

-- name: GetProductAnalyticsDailyAccountScope :one
SELECT *
FROM product_analytics_daily
WHERE seller_account_id = $1
  AND metric_date = $2
  AND scope_type = 'account'
  AND (scope_id = '' OR scope_id = 'account')
ORDER BY CASE WHEN scope_id = '' THEN 0 ELSE 1 END
LIMIT 1;

-- name: ListProductAnalyticsDailyAccountScopeByDateRange :many
SELECT DISTINCT ON (metric_date) *
FROM product_analytics_daily
WHERE seller_account_id = $1
  AND metric_date >= $2
  AND metric_date <= $3
  AND scope_type = 'account'
  AND (scope_id = '' OR scope_id = 'account')
ORDER BY metric_date ASC, CASE WHEN scope_id = '' THEN 0 ELSE 1 END;
