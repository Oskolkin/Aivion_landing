-- SEARCH is already admitted by the domain CHECK constraints in migration 000073.
CREATE TABLE search_queries (
  id BIGSERIAL PRIMARY KEY,
  provider TEXT NOT NULL,
  market TEXT NOT NULL,
  normalized_query_key TEXT NOT NULL CHECK (btrim(normalized_query_key) <> ''),
  display_query TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(provider, market, normalized_query_key)
);

CREATE TABLE search_seller_summaries (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  search_query_id BIGINT NOT NULL REFERENCES search_queries(id) ON DELETE RESTRICT,
  period_start DATE NOT NULL,
  period_end DATE NOT NULL,
  period_kind TEXT NOT NULL CHECK (period_kind IN ('CURRENT_7D','PREVIOUS_7D')),
  provider_search_gmv NUMERIC(30,6) NOT NULL,
  orders NUMERIC(30,6) NOT NULL,
  unique_users NUMERIC(30,6) NOT NULL,
  unique_users_aggregation TEXT NOT NULL CHECK (unique_users_aggregation='NON_ADDITIVE'),
  revision_no BIGINT NOT NULL CHECK (revision_no > 0),
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  observed_at TIMESTAMPTZ NOT NULL,
  source_fragment_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK(period_end >= period_start),
  UNIQUE(seller_account_id,search_query_id,period_start,period_end,state_hash)
);
CREATE INDEX idx_search_summary_cutoff ON search_seller_summaries(seller_account_id,period_start,period_end,observed_at DESC,revision_no DESC,id DESC);

CREATE TABLE search_query_details (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  search_query_id BIGINT NOT NULL REFERENCES search_queries(id) ON DELETE RESTRICT,
  canonical_sku_id BIGINT NULL,
  source_sku BIGINT NOT NULL CHECK(source_sku>0),
  sku_resolution_status TEXT NOT NULL CHECK(sku_resolution_status IN('RESOLVED','UNRESOLVED')),
  provider_rank INTEGER NOT NULL DEFAULT 0 CHECK(provider_rank>=0),
  provider_search_gmv NUMERIC(30,6) NOT NULL,
  orders NUMERIC(30,6) NOT NULL,
  period_start DATE NOT NULL,
  period_end DATE NOT NULL,
  retrieval_scope TEXT NOT NULL CHECK(retrieval_scope='TOP_N_PROVIDER_RANKED'),
  retrieval_reason TEXT NOT NULL,
  top_n_limit INTEGER NOT NULL CHECK(top_n_limit BETWEEN 1 AND 15),
  collection_policy TEXT NOT NULL CHECK(collection_policy='search_detail_collection_policy_v1'),
  revision_no BIGINT NOT NULL CHECK(revision_no>0),
  state_hash TEXT NOT NULL CHECK(state_hash ~ '^[0-9a-f]{64}$'),
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  observed_at TIMESTAMPTZ NOT NULL,
  source_fragment_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  FOREIGN KEY(canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id),
  CHECK((sku_resolution_status='RESOLVED')=(canonical_sku_id IS NOT NULL)),
  UNIQUE(seller_account_id,search_query_id,source_sku,period_start,period_end,state_hash)
);
CREATE INDEX idx_search_details_cutoff ON search_query_details(seller_account_id,search_query_id,period_start,period_end,observed_at DESC,revision_no DESC,id DESC);

CREATE TABLE search_market_facts (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  search_query_id BIGINT NOT NULL REFERENCES search_queries(id) ON DELETE RESTRICT,
  provider_rank INTEGER NOT NULL CHECK(provider_rank>=0),
  period_start DATE NOT NULL,
  period_end DATE NOT NULL,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  observed_at TIMESTAMPTZ NOT NULL,
  UNIQUE(seller_account_id,search_query_id,period_start,period_end,collection_domain_run_id)
);

CREATE OR REPLACE FUNCTION enforce_canonical_search_scope() RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS(SELECT 1 FROM collection_domain_runs d WHERE d.id=NEW.collection_domain_run_id AND d.seller_account_id=NEW.seller_account_id AND d.domain='SEARCH') THEN
    RAISE EXCEPTION 'canonical search domain-run seller mismatch' USING ERRCODE='23514';
  END IF;
  IF NOT EXISTS(SELECT 1 FROM source_payloads p WHERE p.id=NEW.source_payload_id AND p.seller_account_id=NEW.seller_account_id AND p.collection_domain_run_id=NEW.collection_domain_run_id) THEN
    RAISE EXCEPTION 'canonical search payload mismatch' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER search_summary_scope BEFORE INSERT OR UPDATE ON search_seller_summaries FOR EACH ROW EXECUTE FUNCTION enforce_canonical_search_scope();
CREATE TRIGGER search_detail_scope BEFORE INSERT OR UPDATE ON search_query_details FOR EACH ROW EXECUTE FUNCTION enforce_canonical_search_scope();
CREATE TRIGGER search_market_scope BEFORE INSERT OR UPDATE ON search_market_facts FOR EACH ROW EXECUTE FUNCTION enforce_canonical_search_scope();
