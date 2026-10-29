ALTER TABLE collection_authoritative_heads DROP CONSTRAINT collection_authoritative_heads_domain_check;
ALTER TABLE collection_authoritative_heads ADD CONSTRAINT collection_authoritative_heads_domain_check
  CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING','FUNNEL','FINANCE','SEARCH','PROMOTIONS','ACCOUNT_HEALTH','SUPPLY'));

ALTER TABLE data_quality_issues DROP CONSTRAINT data_quality_issues_domain_check;
ALTER TABLE data_quality_issues ADD CONSTRAINT data_quality_issues_domain_check
  CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING','FUNNEL','FINANCE','SEARCH','PROMOTIONS','ACCOUNT_HEALTH','SUPPLY'));

ALTER TABLE business_capability_assessments DROP CONSTRAINT business_capability_assessments_domain_check;
ALTER TABLE business_capability_assessments ADD CONSTRAINT business_capability_assessments_domain_check
  CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING','FUNNEL','FINANCE','SEARCH','PROMOTIONS','ACCOUNT_HEALTH','SUPPLY'));

ALTER TABLE domain_watermarks DROP CONSTRAINT domain_watermarks_domain_check;
ALTER TABLE domain_watermarks ADD CONSTRAINT domain_watermarks_domain_check
  CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING','FUNNEL','FINANCE','SEARCH','PROMOTIONS','ACCOUNT_HEALTH','SUPPLY'));

CREATE TABLE funnel_observations (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  scope_kind TEXT NOT NULL CHECK (scope_kind IN ('ACCOUNT','SKU')),
  canonical_sku_id BIGINT NULL,
  source_sku BIGINT NULL CHECK (source_sku IS NULL OR source_sku > 0),
  sku_resolution_status TEXT NOT NULL CHECK (sku_resolution_status IN ('RESOLVED','UNRESOLVED','NOT_APPLICABLE')),
  metric_key TEXT NOT NULL CHECK (btrim(metric_key) <> ''),
  period_start DATE NOT NULL,
  period_end DATE NOT NULL,
  value_numeric NUMERIC(30,6) NOT NULL,
  unit TEXT NOT NULL CHECK (btrim(unit) <> ''),
  temporal_semantics TEXT NOT NULL CHECK (temporal_semantics IN ('ADDITIVE_FLOW','DIRECT_PERIOD','PROVIDER_RATIO')),
  entity_aggregation_semantics TEXT NOT NULL CHECK (entity_aggregation_semantics IN ('ADDITIVE','NON_ADDITIVE','DIRECT_PROVIDER_GRAIN')),
  surface TEXT NOT NULL CHECK (surface IN ('SEARCH_CATALOG','PDP','CART','UNSPECIFIED')),
  provider_metric_name TEXT NOT NULL CHECK (btrim(provider_metric_name) <> ''),
  registry_version TEXT NOT NULL CHECK (btrim(registry_version) <> ''),
  source_contract_hash TEXT NOT NULL CHECK (source_contract_hash ~ '^[0-9a-f]{64}$'),
  revision_no BIGINT NOT NULL CHECK (revision_no > 0),
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (id, seller_account_id),
  CHECK (period_end >= period_start),
  CHECK (
    (scope_kind = 'ACCOUNT' AND canonical_sku_id IS NULL AND source_sku IS NULL AND sku_resolution_status = 'NOT_APPLICABLE')
    OR (scope_kind = 'SKU' AND source_sku IS NOT NULL AND sku_resolution_status IN ('RESOLVED','UNRESOLVED'))
  ),
  CHECK ((sku_resolution_status = 'RESOLVED') = (canonical_sku_id IS NOT NULL))
);
CREATE UNIQUE INDEX uq_funnel_observations_revision ON funnel_observations (
  seller_account_id, scope_kind, COALESCE(source_sku, 0), metric_key, period_start, period_end, revision_no
);
CREATE UNIQUE INDEX uq_funnel_observations_state_hash ON funnel_observations (
  seller_account_id, scope_kind, COALESCE(source_sku, 0), metric_key, period_start, period_end, state_hash
);
CREATE INDEX idx_funnel_observations_cutoff ON funnel_observations(seller_account_id, metric_key, period_start, period_end, observed_at DESC, revision_no DESC, id DESC);
CREATE INDEX idx_funnel_observations_sku ON funnel_observations(seller_account_id, canonical_sku_id, metric_key, period_start) WHERE canonical_sku_id IS NOT NULL;

CREATE OR REPLACE FUNCTION enforce_funnel_scope() RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM collection_domain_runs dr
    WHERE dr.id = NEW.collection_domain_run_id
      AND dr.seller_account_id = NEW.seller_account_id
      AND dr.domain = 'FUNNEL'
  ) THEN
    RAISE EXCEPTION 'funnel observation domain run seller/domain mismatch' USING ERRCODE = '23514';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM source_payloads sp
    WHERE sp.id = NEW.source_payload_id
      AND sp.seller_account_id = NEW.seller_account_id
      AND sp.collection_domain_run_id = NEW.collection_domain_run_id
  ) THEN
    RAISE EXCEPTION 'funnel observation payload seller/domain-run mismatch' USING ERRCODE = '23514';
  END IF;
  IF NEW.canonical_sku_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM canonical_skus cs
    WHERE cs.id = NEW.canonical_sku_id
      AND cs.seller_account_id = NEW.seller_account_id
      AND cs.ozon_sku = NEW.source_sku
  ) THEN
    RAISE EXCEPTION 'funnel observation canonical SKU seller/source mismatch' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER funnel_observations_scope_trigger
  BEFORE INSERT OR UPDATE ON funnel_observations
  FOR EACH ROW EXECUTE FUNCTION enforce_funnel_scope();
