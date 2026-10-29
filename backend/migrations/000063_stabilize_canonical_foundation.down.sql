DROP TABLE IF EXISTS provider_taxonomy_heads;
DROP TABLE IF EXISTS collection_authoritative_heads;

ALTER TABLE sku_category_assignments
    DROP CONSTRAINT IF EXISTS sku_category_assignments_product_type_resolution_check,
    DROP CONSTRAINT IF EXISTS sku_category_assignments_category_resolution_check,
    DROP COLUMN IF EXISTS product_type_resolution_status,
    DROP COLUMN IF EXISTS category_resolution_status;

ALTER TABLE source_payloads
    DROP COLUMN IF EXISTS redaction_profile;

ALTER TABLE collection_domain_runs
    DROP COLUMN IF EXISTS semantic_mapping_version,
    DROP COLUMN IF EXISTS source_endpoint_family,
    DROP COLUMN IF EXISTS source_contract_version,
    DROP COLUMN IF EXISTS source_contract_key;

ALTER TABLE ozon_connections
    DROP CONSTRAINT IF EXISTS ozon_connections_oauth_state_check,
    DROP CONSTRAINT IF EXISTS ozon_connections_auth_mode_check,
    DROP COLUMN IF EXISTS oauth_last_refresh_error,
    DROP COLUMN IF EXISTS oauth_last_refresh_at,
    DROP COLUMN IF EXISTS oauth_token_updated_at,
    DROP COLUMN IF EXISTS oauth_connection_state,
    DROP COLUMN IF EXISTS oauth_token_type,
    DROP COLUMN IF EXISTS oauth_scopes,
    DROP COLUMN IF EXISTS oauth_access_token_expires_at,
    DROP COLUMN IF EXISTS oauth_refresh_token_encrypted,
    DROP COLUMN IF EXISTS oauth_access_token_encrypted,
    DROP COLUMN IF EXISTS auth_mode;
