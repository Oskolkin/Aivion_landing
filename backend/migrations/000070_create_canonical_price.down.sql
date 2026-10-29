DROP TRIGGER IF EXISTS price_snapshots_scope ON price_snapshots;
DROP TRIGGER IF EXISTS price_collection_snapshots_scope ON price_collection_snapshots;
DROP FUNCTION IF EXISTS enforce_canonical_price_scope();
DROP TABLE IF EXISTS price_snapshots;
DROP TABLE IF EXISTS price_collection_snapshots;

