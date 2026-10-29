ALTER TABLE products
ADD COLUMN reference_price NUMERIC(18,2),
ADD COLUMN old_price NUMERIC(18,2),
ADD COLUMN ozon_min_price NUMERIC(18,2),
ADD COLUMN description_category_id BIGINT;

CREATE INDEX idx_products_reference_price ON products(reference_price);
CREATE INDEX idx_products_old_price ON products(old_price);
CREATE INDEX idx_products_ozon_min_price ON products(ozon_min_price);
CREATE INDEX idx_products_description_category_id ON products(description_category_id);
