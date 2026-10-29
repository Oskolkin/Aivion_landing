ALTER TABLE seller_current_unified_publications
    DROP CONSTRAINT IF EXISTS chk_seller_current_unified_publications_source_mode;

ALTER TABLE seller_current_unified_publications
    ADD CONSTRAINT chk_seller_current_unified_publications_source_mode
    CHECK (source_mode IN ('unified'));

UPDATE seller_current_unified_publications
SET source_mode = 'unified'
WHERE source_mode = 'imported_demo_snapshot';
