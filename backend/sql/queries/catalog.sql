-- name: UpsertCanonicalSKU :one
INSERT INTO canonical_skus (seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)
VALUES ($1,$2,$3,$3,$4)
ON CONFLICT (seller_account_id,ozon_sku) DO UPDATE SET last_observed_at=EXCLUDED.last_observed_at,catalog_state=EXCLUDED.catalog_state,updated_at=NOW()
RETURNING *;
-- name: GetCanonicalSKUForSeller :one
SELECT * FROM canonical_skus WHERE id=$1 AND seller_account_id=$2;
-- name: GetCanonicalSKUByOzonSKUForSeller :one
SELECT * FROM canonical_skus WHERE seller_account_id=$1 AND ozon_sku=$2;
-- name: InsertCanonicalSKUVersion :one
INSERT INTO canonical_sku_versions(canonical_sku_id,provider_product_id,offer_id,name,provider_status,catalog_state,normalized_hash,observed_at,collection_domain_run_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING *;
-- name: GetLatestCanonicalSKUVersion :one
SELECT * FROM canonical_sku_versions WHERE canonical_sku_id=$1 ORDER BY observed_at DESC,id DESC LIMIT 1;
-- name: GetLatestCanonicalSKUContentSnapshot :one
SELECT * FROM canonical_sku_content_snapshots WHERE canonical_sku_id=$1 ORDER BY observed_at DESC,id DESC LIMIT 1;
-- name: InsertCanonicalSKUContentSnapshot :one
INSERT INTO canonical_sku_content_snapshots(canonical_sku_id,primary_image_url,image_urls,normalized_hash,observed_at,collection_domain_run_id) VALUES($1,$2,$3::jsonb,$4,$5,$6) RETURNING *;
-- name: MarkUnobservedCanonicalSKUs :many
UPDATE canonical_skus SET catalog_state='UNOBSERVED',updated_at=NOW() WHERE seller_account_id=$1 AND catalog_state <> 'UNOBSERVED' AND NOT (ozon_sku=ANY(COALESCE($2::bigint[],ARRAY[]::bigint[]))) RETURNING *;

-- name: GetCollectionAuthoritativeHead :one
SELECT * FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain=$2;

-- name: UpsertCollectionAuthoritativeHead :one
INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)
VALUES($1,$2,$3,$4)
ON CONFLICT(seller_account_id,domain) DO UPDATE SET
  collection_domain_run_id=EXCLUDED.collection_domain_run_id,
  finalized_at=EXCLUDED.finalized_at,
  updated_at=NOW()
WHERE collection_authoritative_heads.collection_domain_run_id < EXCLUDED.collection_domain_run_id
RETURNING *;

-- name: GetProviderTaxonomyHead :one
SELECT * FROM provider_taxonomy_heads WHERE provider=$1 AND market=$2;

-- name: UpsertProviderTaxonomyHead :one
INSERT INTO provider_taxonomy_heads(provider,market,collection_domain_run_id,finalized_at)
VALUES($1,$2,$3,$4)
ON CONFLICT(provider,market) DO UPDATE SET
  collection_domain_run_id=EXCLUDED.collection_domain_run_id,
  finalized_at=EXCLUDED.finalized_at,
  updated_at=NOW()
WHERE provider_taxonomy_heads.collection_domain_run_id < EXCLUDED.collection_domain_run_id
RETURNING *;
