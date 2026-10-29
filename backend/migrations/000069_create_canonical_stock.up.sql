ALTER TABLE collection_authoritative_heads DROP CONSTRAINT collection_authoritative_heads_domain_check;
ALTER TABLE collection_authoritative_heads ADD CONSTRAINT collection_authoritative_heads_domain_check
  CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING'));

ALTER TABLE data_quality_issues DROP CONSTRAINT data_quality_issues_domain_check;
ALTER TABLE data_quality_issues ADD CONSTRAINT data_quality_issues_domain_check CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING'));
ALTER TABLE business_capability_assessments DROP CONSTRAINT business_capability_assessments_domain_check;
ALTER TABLE business_capability_assessments ADD CONSTRAINT business_capability_assessments_domain_check CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING'));
ALTER TABLE domain_watermarks DROP CONSTRAINT domain_watermarks_domain_check;
ALTER TABLE domain_watermarks ADD CONSTRAINT domain_watermarks_domain_check CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING'));
ALTER TABLE data_quality_issues DROP CONSTRAINT data_quality_issues_retry_class_check;
ALTER TABLE data_quality_issues ADD CONSTRAINT data_quality_issues_retry_class_check CHECK (retry_class IN ('TRANSIENT','STRUCTURAL','DATA_STATE','PENDING_ACCEPTANCE','CONTRACT_CHANGE'));

CREATE TABLE inventory_pools (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  provider_pool_key TEXT NOT NULL CHECK (btrim(provider_pool_key) <> ''),
  fulfillment_type TEXT NOT NULL CHECK (fulfillment_type IN ('FBO','FBS','RFBS','UNKNOWN')),
  pool_type TEXT NOT NULL CHECK (pool_type IN ('PROVIDER_PHYSICAL','SELLER_WAREHOUSE','UNKNOWN')),
  independence_status TEXT NOT NULL CHECK (independence_status IN ('VERIFIED_INDEPENDENT','SHARED','UNVERIFIED')),
  first_observed_at TIMESTAMPTZ NOT NULL,
  last_observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (seller_account_id,provider_pool_key),
  UNIQUE (id,seller_account_id),
  CHECK (last_observed_at >= first_observed_at)
);

CREATE TABLE inventory_locations (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  provider_location_key TEXT NOT NULL CHECK (btrim(provider_location_key) <> ''),
  location_type TEXT NOT NULL CHECK (location_type IN ('WAREHOUSE','CLUSTER','MACRO_CLUSTER','UNKNOWN')),
  first_observed_at TIMESTAMPTZ NOT NULL,
  last_observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (seller_account_id,provider_location_key),
  UNIQUE (id,seller_account_id),
  CHECK (last_observed_at >= first_observed_at)
);

CREATE TABLE inventory_location_versions (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  inventory_location_id BIGINT NOT NULL,
  version_no BIGINT NOT NULL CHECK (version_no > 0),
  provider_name TEXT NULL,
  parent_location_id BIGINT NULL,
  metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (inventory_location_id,version_no),
  UNIQUE (id,seller_account_id),
  FOREIGN KEY (inventory_location_id,seller_account_id) REFERENCES inventory_locations(id,seller_account_id),
  FOREIGN KEY (parent_location_id,seller_account_id) REFERENCES inventory_locations(id,seller_account_id),
  CHECK (parent_location_id IS NULL OR parent_location_id <> inventory_location_id)
);
CREATE INDEX idx_inventory_location_versions_cutoff ON inventory_location_versions(inventory_location_id,observed_at DESC,id DESC);

CREATE TABLE stock_snapshots (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  snapshot_at TIMESTAMPTZ NOT NULL,
  collection_started_at TIMESTAMPTZ NOT NULL,
  collection_completed_at TIMESTAMPTZ NOT NULL,
  time_quality TEXT NOT NULL CHECK (time_quality IN ('PROVIDER_EXACT','OBSERVED_APPROX')),
  requested_sku_universe_hash TEXT NOT NULL CHECK (requested_sku_universe_hash ~ '^[0-9a-f]{64}$'),
  expected_sku_count BIGINT NOT NULL CHECK (expected_sku_count >= 0),
  requested_sku_count BIGINT NOT NULL CHECK (requested_sku_count >= 0),
  received_sku_count BIGINT NOT NULL CHECK (received_sku_count >= 0),
  resolved_sku_count BIGINT NOT NULL CHECK (resolved_sku_count >= 0),
  unresolved_sku_count BIGINT NOT NULL CHECK (unresolved_sku_count >= 0),
  position_count BIGINT NOT NULL CHECK (position_count >= 0),
  batches_expected BIGINT NOT NULL CHECK (batches_expected >= 0),
  batches_completed BIGINT NOT NULL CHECK (batches_completed >= 0),
  batches_failed BIGINT NOT NULL CHECK (batches_failed >= 0),
  completeness_status TEXT NOT NULL CHECK (completeness_status IN ('COMPLETE','PARTIAL')),
  semantic_mapping_status TEXT NOT NULL CHECK (semantic_mapping_status IN ('VALIDATED','PENDING_ACCEPTANCE','INVALID')),
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (collection_domain_run_id),
  UNIQUE (id,seller_account_id),
  CHECK (collection_completed_at >= collection_started_at),
  CHECK (resolved_sku_count + unresolved_sku_count <= received_sku_count)
);
CREATE INDEX idx_stock_snapshots_cutoff ON stock_snapshots(seller_account_id,snapshot_at DESC,id DESC);

CREATE TABLE stock_snapshot_lines (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  snapshot_id BIGINT NOT NULL,
  canonical_sku_id BIGINT NULL,
  source_sku BIGINT NOT NULL CHECK (source_sku > 0),
  sku_resolution_status TEXT NOT NULL CHECK (sku_resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT')),
  inventory_pool_id BIGINT NOT NULL,
  inventory_location_id BIGINT NULL,
  fulfillment_type TEXT NOT NULL CHECK (fulfillment_type IN ('FBO','FBS','RFBS','UNKNOWN')),
  sellable_units BIGINT NULL CHECK (sellable_units IS NULL OR sellable_units >= 0),
  reserved_units BIGINT NULL CHECK (reserved_units IS NULL OR reserved_units >= 0),
  non_sellable_units BIGINT NULL CHECK (non_sellable_units IS NULL OR non_sellable_units >= 0),
  provider_valid_stock_count BIGINT NULL CHECK (provider_valid_stock_count IS NULL OR provider_valid_stock_count >= 0),
  provider_present_count BIGINT NULL CHECK (provider_present_count IS NULL OR provider_present_count >= 0),
  provider_reserved_count BIGINT NULL CHECK (provider_reserved_count IS NULL OR provider_reserved_count >= 0),
  quantity_semantic_status TEXT NOT NULL CHECK (quantity_semantic_status IN ('VALIDATED','PENDING_ACCEPTANCE','INVALID')),
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (id,seller_account_id),
  FOREIGN KEY (snapshot_id,seller_account_id) REFERENCES stock_snapshots(id,seller_account_id) ON DELETE CASCADE,
  FOREIGN KEY (canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id),
  FOREIGN KEY (inventory_pool_id,seller_account_id) REFERENCES inventory_pools(id,seller_account_id),
  FOREIGN KEY (inventory_location_id,seller_account_id) REFERENCES inventory_locations(id,seller_account_id),
  CHECK ((sku_resolution_status='RESOLVED')=(canonical_sku_id IS NOT NULL))
);
CREATE UNIQUE INDEX stock_snapshot_lines_identity ON stock_snapshot_lines(snapshot_id,source_sku,inventory_pool_id,COALESCE(inventory_location_id,0));
CREATE INDEX idx_stock_snapshot_lines_sku ON stock_snapshot_lines(seller_account_id,canonical_sku_id,snapshot_id) WHERE canonical_sku_id IS NOT NULL;

CREATE TABLE stock_turnover_observations (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  stock_snapshot_line_id BIGINT NOT NULL,
  provider_ads NUMERIC(30,6) NULL,
  provider_ads_cluster NUMERIC(30,6) NULL,
  provider_idc NUMERIC(30,6) NULL,
  provider_idc_cluster NUMERIC(30,6) NULL,
  provider_turnover_grade TEXT NULL,
  provider_turnover_grade_cluster TEXT NULL,
  provider_days_without_sales BIGINT NULL CHECK (provider_days_without_sales IS NULL OR provider_days_without_sales >= 0),
  provider_days_without_sales_cluster BIGINT NULL CHECK (provider_days_without_sales_cluster IS NULL OR provider_days_without_sales_cluster >= 0),
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (stock_snapshot_line_id),
  FOREIGN KEY (stock_snapshot_line_id,seller_account_id) REFERENCES stock_snapshot_lines(id,seller_account_id) ON DELETE CASCADE
);

CREATE OR REPLACE FUNCTION enforce_canonical_stock_scope() RETURNS TRIGGER AS $$
DECLARE run_id BIGINT;
BEGIN
  run_id := CASE WHEN TG_TABLE_NAME='stock_snapshots' THEN NEW.collection_domain_run_id ELSE NEW.collection_domain_run_id END;
  IF NOT EXISTS (SELECT 1 FROM collection_domain_runs dr WHERE dr.id=run_id AND dr.seller_account_id=NEW.seller_account_id AND dr.domain='STOCK') THEN
    RAISE EXCEPTION 'canonical stock domain-run seller mismatch' USING ERRCODE='23514';
  END IF;
  IF TG_TABLE_NAME='inventory_location_versions' THEN
    IF NOT EXISTS (SELECT 1 FROM source_payloads sp WHERE sp.id=NEW.source_payload_id AND sp.seller_account_id=NEW.seller_account_id AND sp.collection_domain_run_id=run_id) THEN
      RAISE EXCEPTION 'canonical stock payload scope mismatch' USING ERRCODE='23514';
    END IF;
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER inventory_location_versions_scope BEFORE INSERT OR UPDATE ON inventory_location_versions FOR EACH ROW EXECUTE FUNCTION enforce_canonical_stock_scope();
CREATE TRIGGER stock_snapshots_scope BEFORE INSERT OR UPDATE ON stock_snapshots FOR EACH ROW EXECUTE FUNCTION enforce_canonical_stock_scope();
