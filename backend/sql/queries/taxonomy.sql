-- name: CreateTaxonomySyncRun :one
INSERT INTO taxonomy_sync_runs(collection_domain_run_id,taxonomy_hash,status,observed_at) VALUES($1,$2,$3,$4) RETURNING *;
-- name: UpsertProviderCategory :one
INSERT INTO provider_categories(provider,provider_category_id,first_observed_at,last_observed_at) VALUES('OZON',$1,$2,$2) ON CONFLICT(provider,provider_category_id) DO UPDATE SET last_observed_at=EXCLUDED.last_observed_at RETURNING *;
-- name: CreateProviderCategoryVersion :one
INSERT INTO provider_category_versions(provider_category_id,parent_provider_category_id,name,disabled,is_leaf,taxonomy_state,taxonomy_sync_run_id,observed_at,normalized_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING *;
-- name: GetLatestProviderCategoryVersion :one
SELECT * FROM provider_category_versions WHERE provider_category_id=$1 ORDER BY observed_at DESC,id DESC LIMIT 1;
-- name: GetProviderCategoryByExternalID :one
SELECT * FROM provider_categories WHERE provider='OZON' AND provider_category_id=$1;
-- name: ListProviderCategories :many
SELECT * FROM provider_categories WHERE provider='OZON' ORDER BY id;
-- name: UpsertProviderProductType :one
INSERT INTO provider_product_types(provider,provider_category_id,provider_type_id,first_observed_at,last_observed_at) VALUES('OZON',$1,$2,$3,$3) ON CONFLICT(provider,provider_category_id,provider_type_id) DO UPDATE SET last_observed_at=EXCLUDED.last_observed_at RETURNING *;
-- name: CreateProviderProductTypeVersion :one
INSERT INTO provider_product_type_versions(provider_product_type_id,provider_category_id,name,disabled,taxonomy_state,taxonomy_sync_run_id,observed_at,normalized_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING *;
-- name: GetLatestProviderProductTypeVersion :one
SELECT * FROM provider_product_type_versions WHERE provider_product_type_id=$1 ORDER BY observed_at DESC,id DESC LIMIT 1;
-- name: GetProviderProductTypeByExternalID :one
SELECT * FROM provider_product_types WHERE provider='OZON' AND provider_category_id=$1 AND provider_type_id=$2;
-- name: ListProviderProductTypes :many
SELECT * FROM provider_product_types WHERE provider='OZON' ORDER BY id;
-- name: UpsertSellerCategory :one
INSERT INTO seller_categories(seller_account_id,provider_category_id,first_observed_at,last_observed_at) VALUES($1,$2,$3,$3) ON CONFLICT(seller_account_id,provider_category_id) DO UPDATE SET last_observed_at=EXCLUDED.last_observed_at,updated_at=NOW() RETURNING *;

-- name: GetCurrentSKUCategoryAssignment :one
SELECT * FROM sku_category_assignments WHERE canonical_sku_id=$1 AND observed_to IS NULL;
-- name: CloseCurrentSKUCategoryAssignment :exec
UPDATE sku_category_assignments SET observed_to=$2 WHERE canonical_sku_id=$1 AND observed_to IS NULL;
-- name: CreateSKUCategoryAssignment :one
INSERT INTO sku_category_assignments(
  canonical_sku_id,seller_category_id,raw_provider_category_id,
  provider_product_type_id,raw_provider_product_type_id,mapping_status,
  category_resolution_status,product_type_resolution_status,
  observed_from,collection_domain_run_id
) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING *;

-- name: CountCanonicalCatalogSummary :one
SELECT
 (SELECT count(*) FROM canonical_skus cs1 WHERE cs1.seller_account_id=sqlc.arg(seller_account_id))::bigint AS canonical_skus,
 (SELECT count(*) FROM canonical_skus cs2 WHERE cs2.seller_account_id=sqlc.arg(seller_account_id) AND cs2.catalog_state='ACTIVE')::bigint AS active,
 (SELECT count(*) FROM canonical_skus cs3 WHERE cs3.seller_account_id=sqlc.arg(seller_account_id) AND cs3.catalog_state='ARCHIVED')::bigint AS archived,
 (SELECT count(*) FROM canonical_skus cs4 WHERE cs4.seller_account_id=sqlc.arg(seller_account_id) AND cs4.catalog_state='UNOBSERVED')::bigint AS unobserved,
 (SELECT count(*) FROM sku_category_assignments a JOIN canonical_skus s ON s.id=a.canonical_sku_id WHERE s.seller_account_id=sqlc.arg(seller_account_id) AND a.observed_to IS NULL AND a.mapping_status='UNMAPPED')::bigint AS unmapped,
 (SELECT count(*) FROM seller_categories sc2 WHERE sc2.seller_account_id=sqlc.arg(seller_account_id))::bigint AS seller_categories,
 (SELECT count(*) FROM sku_category_assignments a2 JOIN canonical_skus s2 ON s2.id=a2.canonical_sku_id WHERE s2.seller_account_id=sqlc.arg(seller_account_id) AND a2.observed_to IS NULL)::bigint AS open_assignments;
