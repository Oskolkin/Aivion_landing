ALTER TABLE ozon_connections
    ADD COLUMN auth_mode TEXT NOT NULL DEFAULT 'LEGACY',
    ADD COLUMN oauth_access_token_encrypted TEXT NULL,
    ADD COLUMN oauth_refresh_token_encrypted TEXT NULL,
    ADD COLUMN oauth_access_token_expires_at TIMESTAMPTZ NULL,
    ADD COLUMN oauth_scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN oauth_token_type TEXT NULL,
    ADD COLUMN oauth_connection_state TEXT NOT NULL DEFAULT 'NOT_CONNECTED',
    ADD COLUMN oauth_token_updated_at TIMESTAMPTZ NULL,
    ADD COLUMN oauth_last_refresh_at TIMESTAMPTZ NULL,
    ADD COLUMN oauth_last_refresh_error TEXT NULL,
    ADD CONSTRAINT ozon_connections_auth_mode_check
        CHECK (auth_mode IN ('LEGACY', 'PRIVATE_OAUTH')),
    ADD CONSTRAINT ozon_connections_oauth_state_check
        CHECK (oauth_connection_state IN ('NOT_CONNECTED', 'CONNECTED', 'REFRESH_ERROR', 'DISCONNECTED'));

COMMENT ON COLUMN ozon_connections.auth_mode IS 'LEGACY is compatibility-only; new canonical collectors require PRIVATE_OAUTH.';
COMMENT ON COLUMN ozon_connections.oauth_access_token_encrypted IS 'Encrypted seller-bound private-application OAuth access token.';
COMMENT ON COLUMN ozon_connections.oauth_refresh_token_encrypted IS 'Encrypted seller-bound private-application OAuth refresh token.';

ALTER TABLE collection_domain_runs
    ADD COLUMN source_contract_key TEXT NULL,
    ADD COLUMN source_contract_version TEXT NULL,
    ADD COLUMN source_endpoint_family TEXT NULL,
    ADD COLUMN semantic_mapping_version TEXT NULL;

ALTER TABLE source_payloads
    ADD COLUMN redaction_profile TEXT NOT NULL DEFAULT 'UNSPECIFIED_V0';

ALTER TABLE sku_category_assignments
    ADD COLUMN category_resolution_status TEXT NOT NULL DEFAULT 'UNRESOLVED',
    ADD COLUMN product_type_resolution_status TEXT NOT NULL DEFAULT 'NOT_APPLICABLE',
    ADD CONSTRAINT sku_category_assignments_category_resolution_check
        CHECK (category_resolution_status IN ('RESOLVED', 'UNRESOLVED', 'NOT_APPLICABLE', 'CONFLICT')),
    ADD CONSTRAINT sku_category_assignments_product_type_resolution_check
        CHECK (product_type_resolution_status IN ('RESOLVED', 'UNRESOLVED', 'NOT_APPLICABLE', 'CONFLICT'));

UPDATE sku_category_assignments
SET
    category_resolution_status = CASE
        WHEN mapping_status = 'MAPPED' THEN 'RESOLVED'
        WHEN raw_provider_category_id IS NULL THEN 'NOT_APPLICABLE'
        ELSE 'UNRESOLVED'
    END,
    product_type_resolution_status = CASE
        WHEN provider_product_type_id IS NOT NULL THEN 'RESOLVED'
        WHEN raw_provider_product_type_id IS NULL THEN 'NOT_APPLICABLE'
        ELSE 'UNRESOLVED'
    END;

CREATE TABLE collection_authoritative_heads (
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE CASCADE,
    finalized_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (seller_account_id, domain),
    CONSTRAINT collection_authoritative_heads_domain_check CHECK (domain IN ('CATALOG'))
);

CREATE TABLE provider_taxonomy_heads (
    provider TEXT NOT NULL,
    market TEXT NOT NULL,
    collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE CASCADE,
    finalized_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider, market),
    CONSTRAINT provider_taxonomy_heads_provider_check CHECK (provider IN ('OZON'))
);
