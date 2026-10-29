-- Fact hashes deduplicate only against the latest accepted revision. A valid
-- knowledge history may return to an older value (A -> B -> A), so historical
-- hash reuse must not be globally unique.
ALTER TABLE sales_demand_daily
    DROP CONSTRAINT sales_demand_daily_fact_unique;

ALTER TABLE sales_account_daily
    DROP CONSTRAINT sales_account_daily_fact_unique;

CREATE INDEX idx_sales_demand_daily_fact_history
    ON sales_demand_daily (seller_account_id, source_sku, business_date, fact_hash, revision_no DESC);

CREATE INDEX idx_sales_account_daily_fact_history
    ON sales_account_daily (seller_account_id, business_date, fact_hash, revision_no DESC);
