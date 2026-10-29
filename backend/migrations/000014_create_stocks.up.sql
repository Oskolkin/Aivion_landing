CREATE TABLE stocks (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    product_external_id TEXT NOT NULL,
    warehouse_external_id TEXT NOT NULL,
    quantity_total INTEGER,
    quantity_reserved INTEGER,
    quantity_available INTEGER,
    snapshot_at TIMESTAMPTZ,
    raw_attributes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_stocks_seller_account_product_warehouse
    ON stocks(seller_account_id, product_external_id, warehouse_external_id);

CREATE INDEX idx_stocks_seller_account_id ON stocks(seller_account_id);
CREATE INDEX idx_stocks_product_external_id ON stocks(product_external_id);
CREATE INDEX idx_stocks_warehouse_external_id ON stocks(warehouse_external_id);
CREATE INDEX idx_stocks_snapshot_at ON stocks(snapshot_at);
CREATE INDEX idx_stocks_updated_at ON stocks(updated_at);