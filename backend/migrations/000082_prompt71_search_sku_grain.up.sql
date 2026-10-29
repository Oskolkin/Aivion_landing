-- Prompt 7.1 Search grain correction:
-- Seller Summary = SELLER × SKU × PERIOD × REVISION (not query-primary).
-- Old query-primary search_seller_summaries rows are wrong-grain; stop writing them.
-- Factual API reads the new SKU summary table only.

CREATE TABLE search_sku_summaries (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  canonical_sku_id BIGINT NULL,
  source_sku BIGINT NOT NULL CHECK (source_sku > 0),
  sku_resolution_status TEXT NOT NULL CHECK (sku_resolution_status IN ('RESOLVED','UNRESOLVED')),
  period_start DATE NOT NULL,
  period_end DATE NOT NULL,
  period_kind TEXT NOT NULL CHECK (period_kind IN ('CURRENT_7D','PREVIOUS_7D')),
  provider_search_gmv NUMERIC(30,6) NOT NULL,
  orders NUMERIC(30,6) NOT NULL,
  unique_search_users NUMERIC(30,6) NULL,
  unique_view_users NUMERIC(30,6) NULL,
  position NUMERIC(30,6) NULL,
  view_conversion NUMERIC(30,6) NULL,
  unique_users_aggregation TEXT NOT NULL DEFAULT 'NON_ADDITIVE' CHECK (unique_users_aggregation = 'NON_ADDITIVE'),
  revision_no BIGINT NOT NULL CHECK (revision_no > 0),
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  observed_at TIMESTAMPTZ NOT NULL,
  source_fragment_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  FOREIGN KEY (canonical_sku_id, seller_account_id) REFERENCES canonical_skus(id, seller_account_id),
  CHECK (period_end >= period_start),
  CHECK ((sku_resolution_status = 'RESOLVED') = (canonical_sku_id IS NOT NULL)),
  UNIQUE (seller_account_id, source_sku, period_start, period_end, revision_no)
);
CREATE INDEX idx_search_sku_summary_cutoff
  ON search_sku_summaries (seller_account_id, source_sku, period_start, period_end, observed_at DESC, revision_no DESC, id DESC);

CREATE TRIGGER search_sku_summary_scope
  BEFORE INSERT OR UPDATE ON search_sku_summaries
  FOR EACH ROW EXECUTE FUNCTION enforce_canonical_search_scope();

-- Immutable first-observed display stays on search_queries.display_query.
-- Mutable later display variants are versioned for AS-KNOWN-AT safety.
CREATE TABLE search_query_versions (
  id BIGSERIAL PRIMARY KEY,
  search_query_id BIGINT NOT NULL REFERENCES search_queries(id) ON DELETE CASCADE,
  display_query TEXT NOT NULL,
  version_no BIGINT NOT NULL CHECK (version_no > 0),
  observed_at TIMESTAMPTZ NOT NULL,
  collection_domain_run_id BIGINT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  source_payload_id BIGINT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (search_query_id, version_no)
);

-- Seed v1 from current display_query without inventing new semantics.
INSERT INTO search_query_versions (search_query_id, display_query, version_no, observed_at)
SELECT id, display_query, 1, created_at FROM search_queries
ON CONFLICT DO NOTHING;

-- Details: keep SKU×Query grain; add retrieval metadata columns if missing.
ALTER TABLE search_query_details
  ADD COLUMN IF NOT EXISTS sort_by TEXT NOT NULL DEFAULT 'orders',
  ADD COLUMN IF NOT EXISTS sort_dir TEXT NOT NULL DEFAULT 'DESC',
  ADD COLUMN IF NOT EXISTS limit_by_sku INTEGER NOT NULL DEFAULT 15 CHECK (limit_by_sku BETWEEN 1 AND 15),
  ADD COLUMN IF NOT EXISTS page INTEGER NOT NULL DEFAULT 1 CHECK (page >= 1),
  ADD COLUMN IF NOT EXISTS page_size INTEGER NOT NULL DEFAULT 100 CHECK (page_size > 0);

COMMENT ON TABLE search_seller_summaries IS 'LEGACY wrong-grain (query-primary). Do not write. Use search_sku_summaries.';
