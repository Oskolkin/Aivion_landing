-- name: UpsertOzonAPICapability :one
INSERT INTO ozon_api_capabilities (
    seller_account_id,
    source,
    captured_at,
    expires_at,
    roles_json,
    methods_json,
    raw_response_json,
    status,
    error_code,
    error_message,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW()
)
ON CONFLICT (seller_account_id, source)
DO UPDATE SET
    captured_at = EXCLUDED.captured_at,
    expires_at = EXCLUDED.expires_at,
    roles_json = EXCLUDED.roles_json,
    methods_json = EXCLUDED.methods_json,
    raw_response_json = EXCLUDED.raw_response_json,
    status = EXCLUDED.status,
    error_code = EXCLUDED.error_code,
    error_message = EXCLUDED.error_message,
    updated_at = NOW()
RETURNING *;

-- name: GetOzonAPICapabilitiesBySeller :many
SELECT *
FROM ozon_api_capabilities
WHERE seller_account_id = $1
ORDER BY source ASC;

-- name: UpsertOzonProductInfoSnapshotByProductID :one
INSERT INTO ozon_product_info_snapshots (
    seller_account_id,
    captured_at,
    snapshot_date,
    product_id,
    sku,
    offer_id,
    name,
    description_category_id,
    type_id,
    currency_code,
    price,
    old_price,
    min_price,
    vat,
    is_archived,
    is_autoarchived,
    is_discounted,
    has_discounted_fbo_item,
    discounted_fbo_stocks,
    is_kgt,
    is_super,
    status,
    status_name,
    status_description,
    status_failed,
    moderate_status,
    validation_status,
    status_updated_at,
    visibility_has_price,
    visibility_has_stock,
    price_index_color,
    external_minimal_price,
    external_price_index_value,
    ozon_minimal_price,
    ozon_price_index_value,
    self_marketplaces_minimal_price,
    self_marketplaces_price_index_value,
    primary_image,
    images_json,
    barcodes_json,
    commissions_json,
    promotions_json,
    errors_json,
    availabilities_json,
    stocks_json,
    raw_response_json,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
    $31, $32, $33, $34, $35, $36, $37, $38, $39, $40,
    $41, $42, $43, $44, $45, $46, NOW()
)
ON CONFLICT (seller_account_id, snapshot_date, product_id)
WHERE product_id IS NOT NULL
DO UPDATE SET
    captured_at = EXCLUDED.captured_at,
    sku = EXCLUDED.sku,
    offer_id = EXCLUDED.offer_id,
    name = EXCLUDED.name,
    description_category_id = EXCLUDED.description_category_id,
    type_id = EXCLUDED.type_id,
    currency_code = EXCLUDED.currency_code,
    price = EXCLUDED.price,
    old_price = EXCLUDED.old_price,
    min_price = EXCLUDED.min_price,
    vat = EXCLUDED.vat,
    is_archived = EXCLUDED.is_archived,
    is_autoarchived = EXCLUDED.is_autoarchived,
    is_discounted = EXCLUDED.is_discounted,
    has_discounted_fbo_item = EXCLUDED.has_discounted_fbo_item,
    discounted_fbo_stocks = EXCLUDED.discounted_fbo_stocks,
    is_kgt = EXCLUDED.is_kgt,
    is_super = EXCLUDED.is_super,
    status = EXCLUDED.status,
    status_name = EXCLUDED.status_name,
    status_description = EXCLUDED.status_description,
    status_failed = EXCLUDED.status_failed,
    moderate_status = EXCLUDED.moderate_status,
    validation_status = EXCLUDED.validation_status,
    status_updated_at = EXCLUDED.status_updated_at,
    visibility_has_price = EXCLUDED.visibility_has_price,
    visibility_has_stock = EXCLUDED.visibility_has_stock,
    price_index_color = EXCLUDED.price_index_color,
    external_minimal_price = EXCLUDED.external_minimal_price,
    external_price_index_value = EXCLUDED.external_price_index_value,
    ozon_minimal_price = EXCLUDED.ozon_minimal_price,
    ozon_price_index_value = EXCLUDED.ozon_price_index_value,
    self_marketplaces_minimal_price = EXCLUDED.self_marketplaces_minimal_price,
    self_marketplaces_price_index_value = EXCLUDED.self_marketplaces_price_index_value,
    primary_image = EXCLUDED.primary_image,
    images_json = EXCLUDED.images_json,
    barcodes_json = EXCLUDED.barcodes_json,
    commissions_json = EXCLUDED.commissions_json,
    promotions_json = EXCLUDED.promotions_json,
    errors_json = EXCLUDED.errors_json,
    availabilities_json = EXCLUDED.availabilities_json,
    stocks_json = EXCLUDED.stocks_json,
    raw_response_json = EXCLUDED.raw_response_json,
    updated_at = NOW()
RETURNING *;

-- name: UpsertOzonProductInfoSnapshotBySKU :one
INSERT INTO ozon_product_info_snapshots (
    seller_account_id,
    captured_at,
    snapshot_date,
    product_id,
    sku,
    offer_id,
    name,
    description_category_id,
    type_id,
    currency_code,
    price,
    old_price,
    min_price,
    vat,
    is_archived,
    is_autoarchived,
    is_discounted,
    has_discounted_fbo_item,
    discounted_fbo_stocks,
    is_kgt,
    is_super,
    status,
    status_name,
    status_description,
    status_failed,
    moderate_status,
    validation_status,
    status_updated_at,
    visibility_has_price,
    visibility_has_stock,
    price_index_color,
    external_minimal_price,
    external_price_index_value,
    ozon_minimal_price,
    ozon_price_index_value,
    self_marketplaces_minimal_price,
    self_marketplaces_price_index_value,
    primary_image,
    images_json,
    barcodes_json,
    commissions_json,
    promotions_json,
    errors_json,
    availabilities_json,
    stocks_json,
    raw_response_json,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
    $31, $32, $33, $34, $35, $36, $37, $38, $39, $40,
    $41, $42, $43, $44, $45, $46, NOW()
)
ON CONFLICT (seller_account_id, snapshot_date, sku)
WHERE product_id IS NULL AND sku IS NOT NULL
DO UPDATE SET
    captured_at = EXCLUDED.captured_at,
    offer_id = EXCLUDED.offer_id,
    name = EXCLUDED.name,
    description_category_id = EXCLUDED.description_category_id,
    type_id = EXCLUDED.type_id,
    currency_code = EXCLUDED.currency_code,
    price = EXCLUDED.price,
    old_price = EXCLUDED.old_price,
    min_price = EXCLUDED.min_price,
    vat = EXCLUDED.vat,
    is_archived = EXCLUDED.is_archived,
    is_autoarchived = EXCLUDED.is_autoarchived,
    is_discounted = EXCLUDED.is_discounted,
    has_discounted_fbo_item = EXCLUDED.has_discounted_fbo_item,
    discounted_fbo_stocks = EXCLUDED.discounted_fbo_stocks,
    is_kgt = EXCLUDED.is_kgt,
    is_super = EXCLUDED.is_super,
    status = EXCLUDED.status,
    status_name = EXCLUDED.status_name,
    status_description = EXCLUDED.status_description,
    status_failed = EXCLUDED.status_failed,
    moderate_status = EXCLUDED.moderate_status,
    validation_status = EXCLUDED.validation_status,
    status_updated_at = EXCLUDED.status_updated_at,
    visibility_has_price = EXCLUDED.visibility_has_price,
    visibility_has_stock = EXCLUDED.visibility_has_stock,
    price_index_color = EXCLUDED.price_index_color,
    external_minimal_price = EXCLUDED.external_minimal_price,
    external_price_index_value = EXCLUDED.external_price_index_value,
    ozon_minimal_price = EXCLUDED.ozon_minimal_price,
    ozon_price_index_value = EXCLUDED.ozon_price_index_value,
    self_marketplaces_minimal_price = EXCLUDED.self_marketplaces_minimal_price,
    self_marketplaces_price_index_value = EXCLUDED.self_marketplaces_price_index_value,
    primary_image = EXCLUDED.primary_image,
    images_json = EXCLUDED.images_json,
    barcodes_json = EXCLUDED.barcodes_json,
    commissions_json = EXCLUDED.commissions_json,
    promotions_json = EXCLUDED.promotions_json,
    errors_json = EXCLUDED.errors_json,
    availabilities_json = EXCLUDED.availabilities_json,
    stocks_json = EXCLUDED.stocks_json,
    raw_response_json = EXCLUDED.raw_response_json,
    updated_at = NOW()
RETURNING *;

-- name: CountOzonProductInfoSnapshotsBySellerAndDate :one
SELECT COUNT(*)::bigint AS count
FROM ozon_product_info_snapshots
WHERE seller_account_id = $1
  AND snapshot_date = $2;

-- name: UpsertOzonRelatedSKU :one
INSERT INTO ozon_related_skus (
    seller_account_id,
    requested_sku,
    related_sku,
    product_id,
    delivery_schema,
    availability,
    deleted_at,
    captured_at,
    raw_response_json,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, NOW()
)
ON CONFLICT (seller_account_id, requested_sku, related_sku)
DO UPDATE SET
    product_id = EXCLUDED.product_id,
    delivery_schema = EXCLUDED.delivery_schema,
    availability = EXCLUDED.availability,
    deleted_at = EXCLUDED.deleted_at,
    captured_at = EXCLUDED.captured_at,
    raw_response_json = EXCLUDED.raw_response_json,
    updated_at = NOW()
RETURNING *;

-- name: InsertOzonRelatedSKUError :one
INSERT INTO ozon_related_sku_errors (
    seller_account_id,
    requested_sku,
    error_sku,
    error_code,
    error_message,
    captured_at
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: CountOzonRelatedSKUsBySeller :one
SELECT COUNT(*)::bigint AS count
FROM ozon_related_skus
WHERE seller_account_id = $1;

-- name: CreateOzonEnrichmentSyncRun :one
INSERT INTO ozon_enrichment_sync_runs (
    seller_account_id,
    metric_date,
    scope,
    status,
    started_at
) VALUES (
    $1, $2, $3, 'failed', $4
)
RETURNING *;

-- name: FinishOzonEnrichmentSyncRun :one
UPDATE ozon_enrichment_sync_runs
SET
    status = $2,
    finished_at = $3,
    seller_api_available = $4,
    performance_api_available = $5,
    product_info_count = $6,
    related_sku_count = $7,
    stock_analytics_count = $8,
    campaign_count = $9,
    campaign_daily_stats_count = $10,
    campaign_objects_count = $11,
    campaign_products_count = $12,
    errors_json = $13,
    warnings_json = $14,
    raw_summary_json = $15
WHERE id = $1
RETURNING *;

-- name: GetLatestOzonEnrichmentSyncRun :one
SELECT *
FROM ozon_enrichment_sync_runs
WHERE seller_account_id = $1
  AND metric_date = $2
  AND scope = $3
ORDER BY started_at DESC
LIMIT 1;

-- name: GetLatestOzonEnrichmentSyncRunBySellerAndDate :one
SELECT *
FROM ozon_enrichment_sync_runs
WHERE seller_account_id = $1
  AND metric_date = $2
ORDER BY started_at DESC
LIMIT 1;

-- name: DeleteOzonEnrichmentSyncRunsBySellerDateScope :exec
DELETE FROM ozon_enrichment_sync_runs
WHERE seller_account_id = $1
  AND metric_date = $2
  AND scope = $3;

-- name: DeleteOzonRelatedSKUErrorsBySellerAndCode :exec
DELETE FROM ozon_related_sku_errors
WHERE seller_account_id = $1
  AND error_code = $2;

-- name: ListAdCampaignsBySellerAccountID :many
SELECT *
FROM ad_campaigns
WHERE seller_account_id = $1
ORDER BY campaign_external_id ASC;

-- name: UpsertAdCampaignObject :one
INSERT INTO ad_campaign_objects (
    seller_account_id,
    campaign_external_id,
    object_external_id,
    object_type,
    captured_at,
    raw_response_json,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, NOW()
)
ON CONFLICT (seller_account_id, campaign_external_id, object_external_id)
DO UPDATE SET
    object_type = EXCLUDED.object_type,
    captured_at = EXCLUDED.captured_at,
    raw_response_json = EXCLUDED.raw_response_json,
    updated_at = NOW()
RETURNING *;

-- name: CountAdCampaignObjectsBySeller :one
SELECT COUNT(*)::bigint AS count
FROM ad_campaign_objects
WHERE seller_account_id = $1;
