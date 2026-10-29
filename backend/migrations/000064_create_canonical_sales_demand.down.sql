DROP TRIGGER IF EXISTS sales_reconciliations_scope_trigger ON sales_reconciliations;
DROP TRIGGER IF EXISTS sales_account_daily_scope_trigger ON sales_account_daily;
DROP TRIGGER IF EXISTS sales_demand_daily_scope_trigger ON sales_demand_daily;
DROP FUNCTION IF EXISTS enforce_sales_demand_scope();
DROP TABLE IF EXISTS sales_reconciliations;
DROP TABLE IF EXISTS sales_account_daily;
DROP TABLE IF EXISTS sales_demand_daily;
ALTER TABLE collection_authoritative_heads DROP CONSTRAINT collection_authoritative_heads_domain_check;
ALTER TABLE collection_authoritative_heads ADD CONSTRAINT collection_authoritative_heads_domain_check CHECK (domain IN ('CATALOG'));
