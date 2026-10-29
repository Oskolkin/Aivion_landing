CREATE TABLE daily_sku_metrics (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    metric_date DATE NOT NULL,
    ozon_product_id BIGINT NOT NULL,
    offer_id TEXT,
    sku BIGINT,
    product_name TEXT,
    revenue NUMERIC(18,2) NOT NULL DEFAULT 0,
    orders_count INTEGER NOT NULL DEFAULT 0,
    stock_available INTEGER NOT NULL DEFAULT 0,
    days_of_cover NUMERIC(10,2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_daily_sku_metrics_seller_date_product
    ON daily_sku_metrics(seller_account_id, metric_date, ozon_product_id);

CREATE INDEX idx_daily_sku_metrics_seller_date
    ON daily_sku_metrics(seller_account_id, metric_date DESC);

CREATE INDEX idx_daily_sku_metrics_seller_product
    ON daily_sku_metrics(seller_account_id, ozon_product_id);
