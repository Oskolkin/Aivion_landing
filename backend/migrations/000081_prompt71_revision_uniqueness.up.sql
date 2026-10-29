-- Prompt 7.1: remove uniqueness that blocks A→B→A revision cycles.
-- Do not rewrite accepted migrations 000073–000078.
-- Constraint names may be truncated to 63 chars by PostgreSQL, so drop by definition.

DROP INDEX IF EXISTS uq_funnel_observations_state_hash;

DO $$
DECLARE cname text;
BEGIN
  FOR cname IN
    SELECT conname FROM pg_constraint
    WHERE conrelid = 'finance_accrual_day_revisions'::regclass
      AND contype = 'u'
      AND pg_get_constraintdef(oid) LIKE '%(seller_account_id, business_date, state_hash)%'
  LOOP
    EXECUTE format('ALTER TABLE finance_accrual_day_revisions DROP CONSTRAINT %I', cname);
  END LOOP;

  FOR cname IN
    SELECT conname FROM pg_constraint
    WHERE conrelid = 'search_seller_summaries'::regclass
      AND contype = 'u'
      AND pg_get_constraintdef(oid) LIKE '%state_hash%'
  LOOP
    EXECUTE format('ALTER TABLE search_seller_summaries DROP CONSTRAINT %I', cname);
  END LOOP;

  FOR cname IN
    SELECT conname FROM pg_constraint
    WHERE conrelid = 'search_query_details'::regclass
      AND contype = 'u'
      AND pg_get_constraintdef(oid) LIKE '%state_hash%'
  LOOP
    EXECUTE format('ALTER TABLE search_query_details DROP CONSTRAINT %I', cname);
  END LOOP;
END $$;

-- Revision uniqueness for details (latest-dedupe is application-level).
CREATE UNIQUE INDEX IF NOT EXISTS uq_search_query_details_revision
  ON search_query_details (seller_account_id, search_query_id, source_sku, period_start, period_end, revision_no);

CREATE UNIQUE INDEX IF NOT EXISTS uq_search_seller_summaries_revision
  ON search_seller_summaries (seller_account_id, search_query_id, period_start, period_end, revision_no);
