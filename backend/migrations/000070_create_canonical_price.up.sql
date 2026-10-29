CREATE TABLE price_collection_snapshots (
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
  batches_expected BIGINT NOT NULL CHECK (batches_expected >= 0),
  batches_completed BIGINT NOT NULL CHECK (batches_completed >= 0),
  batches_failed BIGINT NOT NULL CHECK (batches_failed >= 0),
  completeness_status TEXT NOT NULL CHECK (completeness_status IN ('COMPLETE','PARTIAL')),
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (collection_domain_run_id),
  UNIQUE (id,seller_account_id),
  CHECK (collection_completed_at >= collection_started_at)
);
CREATE INDEX idx_price_collection_snapshots_cutoff ON price_collection_snapshots(seller_account_id,snapshot_at DESC,id DESC);

CREATE TABLE price_snapshots (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  price_collection_snapshot_id BIGINT NOT NULL,
  canonical_sku_id BIGINT NULL,
  source_sku BIGINT NULL CHECK (source_sku IS NULL OR source_sku > 0),
  provider_product_id BIGINT NULL CHECK (provider_product_id IS NULL OR provider_product_id > 0),
  sku_resolution_status TEXT NOT NULL CHECK (sku_resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT')),
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  observed_at TIMESTAMPTZ NOT NULL,
  snapshot_at TIMESTAMPTZ NOT NULL,
  time_quality TEXT NOT NULL CHECK (time_quality IN ('PROVIDER_EXACT','OBSERVED_APPROX')),
  currency_code VARCHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
  provider_current_price NUMERIC(30,6) NULL CHECK (provider_current_price IS NULL OR provider_current_price >= 0),
  current_price_state TEXT NOT NULL CHECK (current_price_state IN ('KNOWN','UNKNOWN','INVALID')),
  seller_promo_price NUMERIC(30,6) NULL CHECK (seller_promo_price IS NULL OR seller_promo_price >= 0),
  customer_price NUMERIC(30,6) NULL CHECK (customer_price IS NULL OR customer_price >= 0),
  old_reference_price NUMERIC(30,6) NULL CHECK (old_reference_price IS NULL OR old_reference_price >= 0),
  minimum_allowed_price NUMERIC(30,6) NULL CHECK (minimum_allowed_price IS NULL OR minimum_allowed_price >= 0),
  retail_price NUMERIC(30,6) NULL CHECK (retail_price IS NULL OR retail_price >= 0),
  vat_rate NUMERIC(12,6) NULL,
  auto_action_enabled BOOLEAN NULL,
  price_index_color TEXT NULL,
  price_index_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  promotion_context_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  semantic_mapping_status TEXT NOT NULL CHECK (semantic_mapping_status IN ('VALIDATED','PENDING_ACCEPTANCE','INVALID')),
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  premium_source_payload_id BIGINT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  reconciliation_status TEXT NOT NULL CHECK (reconciliation_status IN ('NOT_APPLICABLE','PASS','DIFFERENT','UNKNOWN')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (id,seller_account_id),
  FOREIGN KEY (price_collection_snapshot_id,seller_account_id) REFERENCES price_collection_snapshots(id,seller_account_id) ON DELETE CASCADE,
  FOREIGN KEY (canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id),
  CHECK ((sku_resolution_status='RESOLVED')=(canonical_sku_id IS NOT NULL)),
  CHECK ((current_price_state='KNOWN')=(provider_current_price IS NOT NULL))
);
CREATE UNIQUE INDEX price_snapshots_source_identity ON price_snapshots(price_collection_snapshot_id,COALESCE(source_sku,0),COALESCE(provider_product_id,0));
CREATE INDEX idx_price_snapshots_sku_cutoff ON price_snapshots(seller_account_id,canonical_sku_id,observed_at DESC,id DESC) WHERE canonical_sku_id IS NOT NULL;

CREATE OR REPLACE FUNCTION enforce_canonical_price_scope() RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM collection_domain_runs dr WHERE dr.id=NEW.collection_domain_run_id AND dr.seller_account_id=NEW.seller_account_id AND dr.domain='PRICE') THEN
    RAISE EXCEPTION 'canonical price domain-run seller mismatch' USING ERRCODE='23514';
  END IF;
  IF TG_TABLE_NAME='price_snapshots' THEN
    IF NOT EXISTS (SELECT 1 FROM source_payloads sp WHERE sp.id=NEW.source_payload_id AND sp.seller_account_id=NEW.seller_account_id AND sp.collection_domain_run_id=NEW.collection_domain_run_id) THEN
      RAISE EXCEPTION 'canonical price payload scope mismatch' USING ERRCODE='23514';
    END IF;
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER price_collection_snapshots_scope BEFORE INSERT OR UPDATE ON price_collection_snapshots FOR EACH ROW EXECUTE FUNCTION enforce_canonical_price_scope();
CREATE TRIGGER price_snapshots_scope BEFORE INSERT OR UPDATE ON price_snapshots FOR EACH ROW EXECUTE FUNCTION enforce_canonical_price_scope();
