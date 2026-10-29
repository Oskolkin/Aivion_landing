CREATE TABLE products (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    ozon_product_id BIGINT NOT NULL,
    offer_id TEXT,
    sku BIGINT,
    name TEXT NOT NULL,
    status TEXT,
    is_archived BOOLEAN NOT NULL DEFAULT FALSE,
    raw_attributes JSONB,
    source_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_products_seller_account_ozon_product_id
    ON products(seller_account_id, ozon_product_id);

CREATE INDEX idx_products_seller_account_id ON products(seller_account_id);
CREATE INDEX idx_products_offer_id ON products(offer_id);
CREATE INDEX idx_products_sku ON products(sku);
CREATE INDEX idx_products_source_updated_at ON products(source_updated_at);
CREATE INDEX idx_products_updated_at ON products(updated_at);