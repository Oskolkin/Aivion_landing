CREATE TABLE daily_account_metrics (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    metric_date DATE NOT NULL,
    revenue NUMERIC(18,2) NOT NULL DEFAULT 0,
    orders_count INTEGER NOT NULL DEFAULT 0,
    returns_count INTEGER NOT NULL DEFAULT 0,
    cancel_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_daily_account_metrics_seller_date
    ON daily_account_metrics(seller_account_id, metric_date);

CREATE INDEX idx_daily_account_metrics_seller_date
    ON daily_account_metrics(seller_account_id, metric_date DESC);
