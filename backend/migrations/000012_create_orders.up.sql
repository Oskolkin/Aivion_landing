CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    ozon_order_id TEXT NOT NULL,
    posting_number TEXT,
    status TEXT,
    created_at_source TIMESTAMPTZ,
    processed_at_source TIMESTAMPTZ,
    total_amount NUMERIC(18,2),
    currency_code TEXT,
    raw_attributes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_orders_seller_account_ozon_order_id
    ON orders(seller_account_id, ozon_order_id);

CREATE INDEX idx_orders_seller_account_id ON orders(seller_account_id);
CREATE INDEX idx_orders_posting_number ON orders(posting_number);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created_at_source ON orders(created_at_source);
CREATE INDEX idx_orders_processed_at_source ON orders(processed_at_source);
CREATE INDEX idx_orders_updated_at ON orders(updated_at);