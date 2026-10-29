ALTER TABLE products
ADD COLUMN primary_image_url TEXT,
ADD COLUMN image_urls JSONB;

CREATE INDEX idx_products_primary_image_url ON products(primary_image_url)
WHERE primary_image_url IS NOT NULL;
