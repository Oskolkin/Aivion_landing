DROP TRIGGER sales_lifecycle_events_scope ON sales_lifecycle_events;
DROP TRIGGER sales_return_events_scope ON sales_return_events;
DROP TRIGGER sales_posting_relations_scope ON sales_posting_relations;
DROP TRIGGER sales_posting_products_scope ON sales_posting_products;
DROP TRIGGER sales_posting_versions_scope ON sales_posting_versions;
DROP FUNCTION enforce_sales_lifecycle_domain_scope();
DROP FUNCTION enforce_sales_posting_product_scope();
DROP TABLE sales_lifecycle_events;
DROP TABLE sales_return_events;
DROP TABLE sales_returns;
DROP TABLE sales_posting_relations;
DROP TABLE sales_posting_products;
DROP TABLE sales_posting_versions;
DROP TABLE sales_postings;
DROP TABLE sales_orders;
ALTER TABLE canonical_skus DROP CONSTRAINT canonical_skus_id_seller_unique;
-- These pointers only address canonical tables removed by this rollback and
-- cannot remain valid once the lifecycle domains cease to be authoritative.
DELETE FROM collection_authoritative_heads WHERE domain IN ('POSTINGS','RETURNS');
ALTER TABLE collection_authoritative_heads DROP CONSTRAINT collection_authoritative_heads_domain_check;
ALTER TABLE collection_authoritative_heads ADD CONSTRAINT collection_authoritative_heads_domain_check CHECK (domain IN ('CATALOG','SALES_DEMAND'));
