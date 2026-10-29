CREATE TABLE IF NOT EXISTS product_analytics_daily (
    id                  BIGSERIAL PRIMARY KEY,
    seller_account_id   BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    metric_date         DATE NOT NULL,
    scope_type          TEXT NOT NULL DEFAULT 'account',
    scope_id            TEXT NOT NULL DEFAULT '',
    impressions         BIGINT,
    product_views       BIGINT,
    add_to_cart         BIGINT,
    orders_count        BIGINT,
    completed_orders    BIGINT,
    revenue             NUMERIC(18, 2),
    raw_payload_id      BIGINT REFERENCES raw_payloads(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_account_id, metric_date, scope_type, scope_id)
);

CREATE INDEX IF NOT EXISTS idx_product_analytics_daily_seller_date
    ON product_analytics_daily (seller_account_id, metric_date DESC);
