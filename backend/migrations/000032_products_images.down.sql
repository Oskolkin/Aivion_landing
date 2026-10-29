DROP INDEX IF EXISTS idx_products_primary_image_url;

ALTER TABLE products
DROP COLUMN IF EXISTS image_urls,
DROP COLUMN IF EXISTS primary_image_url;
