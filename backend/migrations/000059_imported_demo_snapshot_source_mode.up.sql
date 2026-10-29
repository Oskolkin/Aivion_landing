-- Expand seller_current_unified_publications.source_mode to allow imported demo snapshots
-- used for staging browsable demos without regenerating Unified Analysis via OpenAI.

ALTER TABLE seller_current_unified_publications
    DROP CONSTRAINT IF EXISTS chk_seller_current_unified_publications_source_mode;

ALTER TABLE seller_current_unified_publications
    ADD CONSTRAINT chk_seller_current_unified_publications_source_mode
    CHECK (source_mode IN ('unified', 'imported_demo_snapshot'));
