DROP TRIGGER IF EXISTS stock_snapshots_scope ON stock_snapshots;
DROP TRIGGER IF EXISTS inventory_location_versions_scope ON inventory_location_versions;
DROP FUNCTION IF EXISTS enforce_canonical_stock_scope();
DROP TABLE IF EXISTS stock_turnover_observations;
DROP TABLE IF EXISTS stock_snapshot_lines;
DROP TABLE IF EXISTS stock_snapshots;
DROP TABLE IF EXISTS inventory_location_versions;
DROP TABLE IF EXISTS inventory_locations;
DROP TABLE IF EXISTS inventory_pools;
DELETE FROM collection_authoritative_heads WHERE domain IN ('STOCK','PRICE','ADVERTISING');
ALTER TABLE collection_authoritative_heads DROP CONSTRAINT collection_authoritative_heads_domain_check;
ALTER TABLE collection_authoritative_heads ADD CONSTRAINT collection_authoritative_heads_domain_check
  CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS'));
