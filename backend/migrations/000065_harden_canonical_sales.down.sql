-- Fail closed if valid post-000065 A -> B -> A history exists. Rolling back by
-- deleting or rewriting immutable revisions would corrupt canonical knowledge.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM sales_demand_daily
        GROUP BY seller_account_id, source_sku, business_date, fact_hash
        HAVING COUNT(*) > 1
    ) OR EXISTS (
        SELECT 1 FROM sales_account_daily
        GROUP BY seller_account_id, business_date, fact_hash
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'cannot rollback migration 000065: canonical Sales contains valid repeated historical fact hashes';
    END IF;
END $$;

DROP INDEX idx_sales_account_daily_fact_history;
DROP INDEX idx_sales_demand_daily_fact_history;

ALTER TABLE sales_demand_daily
    ADD CONSTRAINT sales_demand_daily_fact_unique
    UNIQUE (seller_account_id, source_sku, business_date, fact_hash);

ALTER TABLE sales_account_daily
    ADD CONSTRAINT sales_account_daily_fact_unique
    UNIQUE (seller_account_id, business_date, fact_hash);
