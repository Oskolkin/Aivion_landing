DROP INDEX IF EXISTS idx_price_collection_complete;
DROP INDEX IF EXISTS idx_stock_snapshots_complete;
DROP INDEX IF EXISTS idx_ad_spend_recon_cutoff;
DROP INDEX IF EXISTS idx_ad_cpo_cutoff;
DROP INDEX IF EXISTS advertising_cpo_facts_identity_revision;

ALTER TABLE advertising_cpo_facts DROP COLUMN IF EXISTS provider_fact_key;
ALTER TABLE advertising_cpo_facts DROP COLUMN IF EXISTS revision_no;

ALTER TABLE stock_snapshot_lines DROP CONSTRAINT IF EXISTS stock_snapshot_lines_absence_provenance_check;
ALTER TABLE stock_snapshot_lines DROP COLUMN IF EXISTS absence_reason;

-- Restore NOT NULL only when safe; leave nullable if absence rows exist.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM stock_snapshot_lines WHERE source_payload_id IS NULL) THEN
    ALTER TABLE stock_snapshot_lines ALTER COLUMN source_payload_id SET NOT NULL;
  END IF;
END $$;

ALTER TABLE stock_snapshots DROP CONSTRAINT IF EXISTS stock_snapshots_provider_resolution_lte_received_check;
ALTER TABLE stock_snapshots DROP COLUMN IF EXISTS batch_unavailable_sku_count;
ALTER TABLE stock_snapshots DROP COLUMN IF EXISTS missing_response_sku_count;

ALTER TABLE stock_snapshots
  ADD CONSTRAINT stock_snapshots_resolved_sku_count_check1
    CHECK (resolved_sku_count + unresolved_sku_count <= received_sku_count);
