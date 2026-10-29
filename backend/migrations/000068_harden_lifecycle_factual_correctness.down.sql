DROP TRIGGER IF EXISTS sales_posting_relation_versions_scope ON sales_posting_relation_versions;
DROP TABLE IF EXISTS sales_posting_relation_versions;
ALTER TABLE sales_posting_products
    DROP CONSTRAINT IF EXISTS sales_posting_products_missing_sku_check,
    DROP CONSTRAINT IF EXISTS sales_posting_products_source_sku_check;
DO $$ BEGIN
  IF EXISTS(SELECT 1 FROM sales_posting_products WHERE source_sku IS NULL) THEN
    RAISE EXCEPTION 'cannot safely downgrade 000068: posting products with missing source SKU exist';
  END IF;
END $$;
ALTER TABLE sales_posting_products
    ALTER COLUMN source_sku SET NOT NULL,
    ADD CONSTRAINT sales_posting_products_source_sku_check CHECK(source_sku > 0);
ALTER TABLE sales_posting_versions
    DROP CONSTRAINT IF EXISTS sales_posting_versions_order_resolution_consistency,
    DROP CONSTRAINT IF EXISTS sales_posting_versions_order_resolution_check,
    DROP CONSTRAINT IF EXISTS sales_posting_versions_scheme_check,
    DROP CONSTRAINT IF EXISTS sales_posting_versions_order_fk,
    DROP COLUMN IF EXISTS order_resolution_status,
    DROP COLUMN IF EXISTS fulfillment_scheme,
    DROP COLUMN IF EXISTS provider_order_number,
    DROP COLUMN IF EXISTS canonical_order_id;
