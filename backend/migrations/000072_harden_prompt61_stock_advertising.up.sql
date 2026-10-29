-- Prompt 6.1: Stock sparse finalization + Advertising CPO revision / identity hardening.

-- STOCK -----------------------------------------------------------------
ALTER TABLE stock_snapshots DROP CONSTRAINT IF EXISTS stock_snapshots_resolved_sku_count_check1;
ALTER TABLE stock_snapshots DROP CONSTRAINT IF EXISTS stock_snapshots_check1;

DO $$
DECLARE cname text;
BEGIN
  FOR cname IN
    SELECT conname FROM pg_constraint
    WHERE conrelid = 'stock_snapshots'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%resolved_sku_count + unresolved_sku_count <= received_sku_count%'
  LOOP
    EXECUTE format('ALTER TABLE stock_snapshots DROP CONSTRAINT %I', cname);
  END LOOP;
END $$;

ALTER TABLE stock_snapshots
  ADD COLUMN IF NOT EXISTS missing_response_sku_count BIGINT NOT NULL DEFAULT 0 CHECK (missing_response_sku_count >= 0),
  ADD COLUMN IF NOT EXISTS batch_unavailable_sku_count BIGINT NOT NULL DEFAULT 0 CHECK (batch_unavailable_sku_count >= 0);

ALTER TABLE stock_snapshots
  ADD CONSTRAINT stock_snapshots_provider_resolution_lte_received_check
    CHECK (resolved_sku_count + unresolved_sku_count <= received_sku_count);

ALTER TABLE stock_snapshot_lines
  ALTER COLUMN source_payload_id DROP NOT NULL;

ALTER TABLE stock_snapshot_lines
  ADD COLUMN IF NOT EXISTS absence_reason TEXT NULL
    CHECK (absence_reason IS NULL OR absence_reason IN ('MISSING_PROVIDER_RESPONSE_ROW','SOURCE_BATCH_UNAVAILABLE'));

ALTER TABLE stock_snapshot_lines
  ADD CONSTRAINT stock_snapshot_lines_absence_provenance_check
    CHECK (
      (absence_reason IS NULL AND source_payload_id IS NOT NULL)
      OR (absence_reason IS NOT NULL AND source_payload_id IS NULL)
    );

-- ADVERTISING CPO revisions --------------------------------------------
ALTER TABLE advertising_cpo_facts
  ADD COLUMN IF NOT EXISTS revision_no BIGINT NOT NULL DEFAULT 1 CHECK (revision_no > 0),
  ADD COLUMN IF NOT EXISTS provider_fact_key TEXT NOT NULL DEFAULT '';

UPDATE advertising_cpo_facts
SET provider_fact_key = md5(
  COALESCE(advertising_campaign_id::text,'') || '|' ||
  COALESCE(business_date::text,'') || '|' ||
  COALESCE(source_order_id::text,'') || '|' ||
  COALESCE(source_promoted_sku::text,'') || '|' ||
  COALESCE(source_ordered_sku::text,'') || '|' ||
  COALESCE(attribution_kind,'') || '|' ||
  id::text
)
WHERE provider_fact_key = '';

ALTER TABLE advertising_cpo_facts
  ALTER COLUMN provider_fact_key DROP DEFAULT;

CREATE UNIQUE INDEX IF NOT EXISTS advertising_cpo_facts_identity_revision
  ON advertising_cpo_facts (seller_account_id, provider_fact_key, revision_no);

CREATE INDEX IF NOT EXISTS idx_ad_cpo_cutoff
  ON advertising_cpo_facts (seller_account_id, provider_fact_key, observed_at DESC, revision_no DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_ad_spend_recon_cutoff
  ON advertising_spend_reconciliations (seller_account_id, business_date, observed_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_stock_snapshots_complete
  ON stock_snapshots (seller_account_id, completeness_status, snapshot_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_price_collection_complete
  ON price_collection_snapshots (seller_account_id, completeness_status, snapshot_at DESC, id DESC);
