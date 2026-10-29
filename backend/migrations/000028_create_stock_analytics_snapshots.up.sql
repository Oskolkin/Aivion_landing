CREATE TABLE stock_analytics_snapshots (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    snapshot_date DATE NOT NULL,
    sku BIGINT NOT NULL,
    offer_id TEXT,
    product_name TEXT,
    warehouse_id BIGINT NOT NULL DEFAULT 0,
    warehouse_name TEXT,
    available_stock_count INTEGER NOT NULL DEFAULT 0,
    excess_stock_count INTEGER NOT NULL DEFAULT 0,
    ads NUMERIC(18, 4),
    idc INTEGER,
    turnover_grade TEXT,
    turnover_grade_cluster TEXT,
    days_without_sales INTEGER,
    requested_stock_count INTEGER,
    transit_stock_count INTEGER,
    valid_stock_count INTEGER,
    waiting_docs_stock_count INTEGER,
    return_from_customer_stock_count INTEGER,
    return_to_seller_stock_count INTEGER,
    raw_payload_id BIGINT REFERENCES raw_payloads(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_stock_analytics_snapshots_seller_date_sku_wh
    ON stock_analytics_snapshots(seller_account_id, snapshot_date, sku, warehouse_id);

CREATE INDEX idx_stock_analytics_snapshots_seller_date
    ON stock_analytics_snapshots(seller_account_id, snapshot_date);

CREATE INDEX idx_stock_analytics_snapshots_seller_sku_date
    ON stock_analytics_snapshots(seller_account_id, sku, snapshot_date);

CREATE INDEX idx_stock_analytics_snapshots_seller_turnover_date
    ON stock_analytics_snapshots(seller_account_id, turnover_grade, snapshot_date);

CREATE INDEX idx_stock_analytics_snapshots_seller_idc_date
    ON stock_analytics_snapshots(seller_account_id, idc, snapshot_date);
