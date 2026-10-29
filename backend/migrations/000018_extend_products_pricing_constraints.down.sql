DROP INDEX IF EXISTS idx_products_description_category_id;
DROP INDEX IF EXISTS idx_products_ozon_min_price;
DROP INDEX IF EXISTS idx_products_old_price;
DROP INDEX IF EXISTS idx_products_reference_price;

ALTER TABLE products
DROP COLUMN IF EXISTS description_category_id,
DROP COLUMN IF EXISTS ozon_min_price,
DROP COLUMN IF EXISTS old_price,
DROP COLUMN IF EXISTS reference_price;
