CREATE TABLE sales (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    ozon_sale_id TEXT NOT NULL,
    ozon_order_id TEXT,
    posting_number TEXT,
    quantity INTEGER,
    amount NUMERIC(18,2),
    currency_code TEXT,
    sale_date TIMESTAMPTZ,
    raw_attributes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_sales_seller_account_ozon_sale_id
    ON sales(seller_account_id, ozon_sale_id);

CREATE INDEX idx_sales_seller_account_id ON sales(seller_account_id);
CREATE INDEX idx_sales_ozon_order_id ON sales(ozon_order_id);
CREATE INDEX idx_sales_posting_number ON sales(posting_number);
CREATE INDEX idx_sales_sale_date ON sales(sale_date);
CREATE INDEX idx_sales_updated_at ON sales(updated_at);