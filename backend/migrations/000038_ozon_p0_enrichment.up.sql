-- P0 Ozon enrichment: API capabilities, product snapshots, related SKUs, sync diagnostics,
-- extended stock analytics, advertising enrichment.

CREATE TABLE ozon_api_capabilities (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NULL,
    roles_json JSONB NOT NULL DEFAULT '[]',
    methods_json JSONB NOT NULL DEFAULT '[]',
    raw_response_json JSONB NULL,
    status TEXT NOT NULL,
    error_code TEXT NULL,
    error_message TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_ozon_api_capabilities_source CHECK (source IN ('seller_api', 'performance_api')),
    CONSTRAINT chk_ozon_api_capabilities_status CHECK (
        status IN ('available', 'error', 'unauthorized', 'forbidden', 'skipped')
    )
);

CREATE UNIQUE INDEX uq_ozon_api_capabilities_seller_source
    ON ozon_api_capabilities(seller_account_id, source);

CREATE INDEX idx_ozon_api_capabilities_seller
    ON ozon_api_capabilities(seller_account_id);

CREATE TABLE ozon_product_info_snapshots (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    captured_at TIMESTAMPTZ NOT NULL,
    snapshot_date DATE NOT NULL,
    product_id BIGINT NULL,
    sku BIGINT NULL,
    offer_id TEXT NULL,
    name TEXT NULL,
    description_category_id BIGINT NULL,
    type_id BIGINT NULL,
    currency_code TEXT NULL,
    price NUMERIC(18, 2) NULL,
    old_price NUMERIC(18, 2) NULL,
    min_price NUMERIC(18, 2) NULL,
    vat TEXT NULL,
    is_archived BOOLEAN NULL,
    is_autoarchived BOOLEAN NULL,
    is_discounted BOOLEAN NULL,
    has_discounted_fbo_item BOOLEAN NULL,
    discounted_fbo_stocks INT NULL,
    is_kgt BOOLEAN NULL,
    is_super BOOLEAN NULL,
    status TEXT NULL,
    status_name TEXT NULL,
    status_description TEXT NULL,
    status_failed TEXT NULL,
    moderate_status TEXT NULL,
    validation_status TEXT NULL,
    status_updated_at TIMESTAMPTZ NULL,
    visibility_has_price BOOLEAN NULL,
    visibility_has_stock BOOLEAN NULL,
    price_index_color TEXT NULL,
    external_minimal_price NUMERIC(18, 2) NULL,
    external_price_index_value NUMERIC(18, 4) NULL,
    ozon_minimal_price NUMERIC(18, 2) NULL,
    ozon_price_index_value NUMERIC(18, 4) NULL,
    self_marketplaces_minimal_price NUMERIC(18, 2) NULL,
    self_marketplaces_price_index_value NUMERIC(18, 4) NULL,
    primary_image TEXT NULL,
    images_json JSONB NOT NULL DEFAULT '[]',
    barcodes_json JSONB NOT NULL DEFAULT '[]',
    commissions_json JSONB NOT NULL DEFAULT '[]',
    promotions_json JSONB NOT NULL DEFAULT '[]',
    errors_json JSONB NOT NULL DEFAULT '[]',
    availabilities_json JSONB NOT NULL DEFAULT '[]',
    stocks_json JSONB NOT NULL DEFAULT '{}',
    raw_response_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_ozon_product_info_snapshots_seller_date_product
    ON ozon_product_info_snapshots(seller_account_id, snapshot_date, product_id)
    WHERE product_id IS NOT NULL;

CREATE UNIQUE INDEX uq_ozon_product_info_snapshots_seller_date_sku
    ON ozon_product_info_snapshots(seller_account_id, snapshot_date, sku)
    WHERE product_id IS NULL AND sku IS NOT NULL;

CREATE INDEX idx_ozon_product_info_snapshots_seller_date
    ON ozon_product_info_snapshots(seller_account_id, snapshot_date DESC);

CREATE TABLE ozon_related_skus (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    requested_sku BIGINT NOT NULL,
    related_sku BIGINT NOT NULL,
    product_id BIGINT NULL,
    delivery_schema TEXT NULL,
    availability TEXT NULL,
    deleted_at TIMESTAMPTZ NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    raw_response_json JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_ozon_related_skus_seller_requested_related
    ON ozon_related_skus(seller_account_id, requested_sku, related_sku);

CREATE INDEX idx_ozon_related_skus_seller
    ON ozon_related_skus(seller_account_id);

CREATE TABLE ozon_related_sku_errors (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    requested_sku BIGINT NOT NULL,
    error_sku BIGINT NULL,
    error_code TEXT NULL,
    error_message TEXT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ozon_related_sku_errors_seller
    ON ozon_related_sku_errors(seller_account_id);

CREATE TABLE ozon_enrichment_sync_runs (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    metric_date DATE NOT NULL,
    scope TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NULL,
    seller_api_available BOOLEAN NOT NULL DEFAULT FALSE,
    performance_api_available BOOLEAN NOT NULL DEFAULT FALSE,
    product_info_count INT NOT NULL DEFAULT 0,
    related_sku_count INT NOT NULL DEFAULT 0,
    stock_analytics_count INT NOT NULL DEFAULT 0,
    campaign_count INT NOT NULL DEFAULT 0,
    campaign_daily_stats_count INT NOT NULL DEFAULT 0,
    campaign_objects_count INT NOT NULL DEFAULT 0,
    campaign_products_count INT NOT NULL DEFAULT 0,
    errors_json JSONB NOT NULL DEFAULT '[]',
    warnings_json JSONB NOT NULL DEFAULT '[]',
    raw_summary_json JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_ozon_enrichment_sync_runs_status CHECK (
        status IN ('completed', 'completed_partial', 'failed')
    )
);

CREATE INDEX idx_ozon_enrichment_sync_runs_seller_date
    ON ozon_enrichment_sync_runs(seller_account_id, metric_date DESC);

CREATE TABLE ad_campaign_objects (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    campaign_external_id BIGINT NOT NULL,
    object_external_id TEXT NOT NULL,
    object_type TEXT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    raw_response_json JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_ad_campaign_objects_campaign
        FOREIGN KEY (seller_account_id, campaign_external_id)
        REFERENCES ad_campaigns(seller_account_id, campaign_external_id)
        ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_ad_campaign_objects_seller_campaign_object
    ON ad_campaign_objects(seller_account_id, campaign_external_id, object_external_id);

CREATE INDEX idx_ad_campaign_objects_seller
    ON ad_campaign_objects(seller_account_id);

-- Extend stock_analytics_snapshots for full P0 analytics fields.
ALTER TABLE stock_analytics_snapshots
    ADD COLUMN IF NOT EXISTS cluster_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cluster_name TEXT,
    ADD COLUMN IF NOT EXISTS macrolocal_cluster_id BIGINT,
    ADD COLUMN IF NOT EXISTS ads_cluster NUMERIC(18, 4),
    ADD COLUMN IF NOT EXISTS idc_cluster NUMERIC(18, 4),
    ADD COLUMN IF NOT EXISTS days_without_sales_cluster INT,
    ADD COLUMN IF NOT EXISTS expiring_stock_count INT,
    ADD COLUMN IF NOT EXISTS other_stock_count INT,
    ADD COLUMN IF NOT EXISTS stock_defect_stock_count INT,
    ADD COLUMN IF NOT EXISTS transit_defect_stock_count INT,
    ADD COLUMN IF NOT EXISTS item_tags_json JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS captured_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS raw_response_json JSONB;

DROP INDEX IF EXISTS uq_stock_analytics_snapshots_seller_date_sku_wh;

CREATE UNIQUE INDEX uq_stock_analytics_snapshots_seller_date_sku_wh_cluster
    ON stock_analytics_snapshots(seller_account_id, snapshot_date, sku, warehouse_id, cluster_id);

-- Extend ad_campaigns for Performance API enrichment.
ALTER TABLE ad_campaigns
    ADD COLUMN IF NOT EXISTS payment_type TEXT,
    ADD COLUMN IF NOT EXISTS placement_json JSONB,
    ADD COLUMN IF NOT EXISTS budget_raw_micro BIGINT,
    ADD COLUMN IF NOT EXISTS daily_budget_raw_micro BIGINT,
    ADD COLUMN IF NOT EXISTS weekly_budget_raw_micro BIGINT,
    ADD COLUMN IF NOT EXISTS budget_weekly NUMERIC(18, 2),
    ADD COLUMN IF NOT EXISTS from_date DATE,
    ADD COLUMN IF NOT EXISTS to_date DATE,
    ADD COLUMN IF NOT EXISTS autostop_status TEXT,
    ADD COLUMN IF NOT EXISTS is_autocreated BOOLEAN,
    ADD COLUMN IF NOT EXISTS product_autopilot_strategy TEXT,
    ADD COLUMN IF NOT EXISTS product_campaign_mode TEXT,
    ADD COLUMN IF NOT EXISTS performance_created_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS performance_updated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS captured_at TIMESTAMPTZ;

-- Extend ad_metrics_daily.
ALTER TABLE ad_metrics_daily
    ADD COLUMN IF NOT EXISTS campaign_name TEXT;

-- Extend ad_campaign_skus for bid enrichment.
ALTER TABLE ad_campaign_skus
    ADD COLUMN IF NOT EXISTS product_title TEXT,
    ADD COLUMN IF NOT EXISTS bid_raw_micro BIGINT,
    ADD COLUMN IF NOT EXISTS bid_amount NUMERIC(18, 2),
    ADD COLUMN IF NOT EXISTS bid_unit_assumption TEXT,
    ADD COLUMN IF NOT EXISTS captured_at TIMESTAMPTZ;
