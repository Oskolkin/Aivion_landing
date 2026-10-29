CREATE TABLE promotions (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  promotion_origin TEXT NOT NULL CHECK (promotion_origin IN ('OZON','SELLER')),
  provider_action_id BIGINT NOT NULL CHECK (provider_action_id > 0),
  first_observed_at TIMESTAMPTZ NOT NULL,
  last_observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (seller_account_id,promotion_origin,provider_action_id),
  UNIQUE (id,seller_account_id),
  CHECK (last_observed_at >= first_observed_at)
);

CREATE TABLE promotion_versions (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  promotion_id BIGINT NOT NULL,
  version_no BIGINT NOT NULL CHECK (version_no > 0),
  title TEXT NULL,
  provider_state TEXT NOT NULL,
  starts_at TIMESTAMPTZ NULL,
  ends_at TIMESTAMPTZ NULL,
  discount_type TEXT NULL,
  discount_value NUMERIC(30,6) NULL,
  currency_code VARCHAR(3) NULL CHECK (currency_code IS NULL OR currency_code ~ '^[A-Z]{3}$'),
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (promotion_id,version_no),
  UNIQUE (id,seller_account_id),
  FOREIGN KEY (promotion_id,seller_account_id) REFERENCES promotions(id,seller_account_id)
);
CREATE INDEX idx_promotion_versions_cutoff ON promotion_versions(promotion_id,observed_at DESC,version_no DESC,id DESC);

CREATE TABLE promotion_rule_components (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  promotion_version_id BIGINT NOT NULL,
  component_ordinal INTEGER NOT NULL CHECK (component_ordinal > 0),
  component_type TEXT NOT NULL,
  component_json JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (promotion_version_id,component_ordinal),
  FOREIGN KEY (promotion_version_id,seller_account_id) REFERENCES promotion_versions(id,seller_account_id) ON DELETE CASCADE
);

CREATE TABLE promotion_sku_memberships (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  promotion_id BIGINT NOT NULL,
  canonical_sku_id BIGINT NULL,
  source_product_id BIGINT NULL CHECK (source_product_id IS NULL OR source_product_id > 0),
  source_offer_id TEXT NULL,
  source_sku BIGINT NULL CHECK (source_sku IS NULL OR source_sku > 0),
  sku_resolution_status TEXT NOT NULL CHECK (sku_resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT')),
  action_price NUMERIC(30,6) NULL CHECK (action_price IS NULL OR action_price >= 0),
  action_stock BIGINT NULL CHECK (action_stock IS NULL OR action_stock >= 0),
  known_from TIMESTAMPTZ NOT NULL,
  known_to TIMESTAMPTZ NULL,
  time_quality TEXT NOT NULL CHECK (time_quality='OBSERVED_APPROX'),
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  FOREIGN KEY (promotion_id,seller_account_id) REFERENCES promotions(id,seller_account_id),
  FOREIGN KEY (canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id),
  CHECK (known_to IS NULL OR known_to >= known_from),
  CHECK ((sku_resolution_status='RESOLVED')=(canonical_sku_id IS NOT NULL)),
  CHECK (source_product_id IS NOT NULL OR source_offer_id IS NOT NULL OR source_sku IS NOT NULL)
);
CREATE UNIQUE INDEX promotion_memberships_open_identity
  ON promotion_sku_memberships(promotion_id,COALESCE(source_sku,0),COALESCE(source_product_id,0),COALESCE(source_offer_id,''))
  WHERE known_to IS NULL;

CREATE TABLE promotion_eligibility_snapshot_runs (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  promotion_id BIGINT NOT NULL,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  observed_at TIMESTAMPTZ NOT NULL,
  completeness_status TEXT NOT NULL CHECK (completeness_status IN ('COMPLETE','PARTIAL')),
  rows_received BIGINT NOT NULL DEFAULT 0 CHECK (rows_received >= 0),
  source_payload_id BIGINT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (promotion_id,collection_domain_run_id),
  UNIQUE (id,seller_account_id),
  FOREIGN KEY (promotion_id,seller_account_id) REFERENCES promotions(id,seller_account_id)
);

CREATE TABLE promotion_sku_eligibility_snapshots (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  snapshot_run_id BIGINT NOT NULL,
  canonical_sku_id BIGINT NULL,
  source_product_id BIGINT NULL CHECK (source_product_id IS NULL OR source_product_id > 0),
  source_offer_id TEXT NULL,
  source_sku BIGINT NULL CHECK (source_sku IS NULL OR source_sku > 0),
  sku_resolution_status TEXT NOT NULL CHECK (sku_resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT')),
  eligibility_state TEXT NOT NULL CHECK (eligibility_state='CANDIDATE'),
  action_price NUMERIC(30,6) NULL CHECK (action_price IS NULL OR action_price >= 0),
  action_stock BIGINT NULL CHECK (action_stock IS NULL OR action_stock >= 0),
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  FOREIGN KEY (canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id),
  FOREIGN KEY (snapshot_run_id,seller_account_id) REFERENCES promotion_eligibility_snapshot_runs(id,seller_account_id) ON DELETE CASCADE,
  CHECK ((sku_resolution_status='RESOLVED')=(canonical_sku_id IS NOT NULL)),
  CHECK (source_product_id IS NOT NULL OR source_offer_id IS NOT NULL OR source_sku IS NOT NULL)
);

CREATE OR REPLACE FUNCTION enforce_canonical_promotions_scope() RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM collection_domain_runs dr WHERE dr.id=NEW.collection_domain_run_id AND dr.seller_account_id=NEW.seller_account_id AND dr.domain='PROMOTIONS') THEN
    RAISE EXCEPTION 'canonical promotions domain-run seller mismatch' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER promotion_versions_scope BEFORE INSERT OR UPDATE ON promotion_versions FOR EACH ROW EXECUTE FUNCTION enforce_canonical_promotions_scope();
CREATE TRIGGER promotion_memberships_scope BEFORE INSERT OR UPDATE ON promotion_sku_memberships FOR EACH ROW EXECUTE FUNCTION enforce_canonical_promotions_scope();
CREATE TRIGGER promotion_eligibility_runs_scope BEFORE INSERT OR UPDATE ON promotion_eligibility_snapshot_runs FOR EACH ROW EXECUTE FUNCTION enforce_canonical_promotions_scope();
