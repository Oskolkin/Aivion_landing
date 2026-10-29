CREATE TABLE ad_campaigns (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    campaign_external_id BIGINT NOT NULL,
    campaign_name TEXT NOT NULL,
    campaign_type TEXT,
    placement_type TEXT,
    status TEXT,
    budget_amount NUMERIC(18,2),
    budget_daily NUMERIC(18,2),
    raw_attributes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_ad_campaigns_seller_campaign_external
    ON ad_campaigns(seller_account_id, campaign_external_id);

CREATE INDEX idx_ad_campaigns_seller_account_id
    ON ad_campaigns(seller_account_id);

CREATE INDEX idx_ad_campaigns_campaign_external_id
    ON ad_campaigns(campaign_external_id);

CREATE INDEX idx_ad_campaigns_status
    ON ad_campaigns(status);

CREATE TABLE ad_metrics_daily (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    campaign_external_id BIGINT NOT NULL,
    metric_date DATE NOT NULL,
    impressions BIGINT NOT NULL DEFAULT 0,
    clicks BIGINT NOT NULL DEFAULT 0,
    spend NUMERIC(18,2) NOT NULL DEFAULT 0,
    orders_count INTEGER NOT NULL DEFAULT 0,
    revenue NUMERIC(18,2) NOT NULL DEFAULT 0,
    raw_attributes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_ad_metrics_daily_campaign
        FOREIGN KEY (seller_account_id, campaign_external_id)
        REFERENCES ad_campaigns(seller_account_id, campaign_external_id)
        ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_ad_metrics_daily_seller_campaign_date
    ON ad_metrics_daily(seller_account_id, campaign_external_id, metric_date);

CREATE INDEX idx_ad_metrics_daily_seller_account_id
    ON ad_metrics_daily(seller_account_id);

CREATE INDEX idx_ad_metrics_daily_campaign_external_id
    ON ad_metrics_daily(campaign_external_id);

CREATE INDEX idx_ad_metrics_daily_metric_date
    ON ad_metrics_daily(metric_date DESC);

CREATE INDEX idx_ad_metrics_daily_seller_metric_date
    ON ad_metrics_daily(seller_account_id, metric_date DESC);

CREATE INDEX idx_ad_metrics_daily_campaign_metric_date
    ON ad_metrics_daily(campaign_external_id, metric_date DESC);

CREATE TABLE ad_campaign_skus (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    campaign_external_id BIGINT NOT NULL,
    ozon_product_id BIGINT NOT NULL,
    offer_id TEXT,
    sku BIGINT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    status TEXT,
    raw_attributes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_ad_campaign_skus_campaign
        FOREIGN KEY (seller_account_id, campaign_external_id)
        REFERENCES ad_campaigns(seller_account_id, campaign_external_id)
        ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_ad_campaign_skus_seller_campaign_product
    ON ad_campaign_skus(seller_account_id, campaign_external_id, ozon_product_id);

CREATE INDEX idx_ad_campaign_skus_seller_account_id
    ON ad_campaign_skus(seller_account_id);

CREATE INDEX idx_ad_campaign_skus_campaign_external_id
    ON ad_campaign_skus(campaign_external_id);

CREATE INDEX idx_ad_campaign_skus_ozon_product_id
    ON ad_campaign_skus(ozon_product_id);

CREATE INDEX idx_ad_campaign_skus_seller_ozon_product
    ON ad_campaign_skus(seller_account_id, ozon_product_id);
