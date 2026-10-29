CREATE TABLE supply_state_registry_versions (
  id BIGSERIAL PRIMARY KEY,
  provider TEXT NOT NULL,
  mapping_version TEXT NOT NULL,
  provider_state_code TEXT NOT NULL,
  canonical_state TEXT NOT NULL,
  valid_from TIMESTAMPTZ NOT NULL,
  valid_to TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(provider,mapping_version,provider_state_code,valid_from),
  CHECK(valid_to IS NULL OR valid_to>valid_from)
);
INSERT INTO supply_state_registry_versions(provider,mapping_version,provider_state_code,canonical_state,valid_from) VALUES
('OZON','supply_state_mapping_v1','CREATED','CREATED','2026-01-01T00:00:00Z'),
('OZON','supply_state_mapping_v1','DATA_FILLING','PLANNING','2026-01-01T00:00:00Z'),
('OZON','supply_state_mapping_v1','READY_TO_SUPPLY','READY','2026-01-01T00:00:00Z'),
('OZON','supply_state_mapping_v1','ACCEPTED_AT_SUPPLY_WAREHOUSE','ACCEPTING','2026-01-01T00:00:00Z'),
('OZON','supply_state_mapping_v1','IN_TRANSIT','IN_TRANSIT','2026-01-01T00:00:00Z'),
('OZON','supply_state_mapping_v1','ACCEPTANCE_AT_STORAGE_WAREHOUSE','ACCEPTING','2026-01-01T00:00:00Z'),
('OZON','supply_state_mapping_v1','REPORTS_CONFIRMATION_AWAITING','ACCEPTING','2026-01-01T00:00:00Z'),
('OZON','supply_state_mapping_v1','COMPLETED','COMPLETED','2026-01-01T00:00:00Z'),
('OZON','supply_state_mapping_v1','CANCELLED','CANCELLED','2026-01-01T00:00:00Z'),
('OZON','supply_state_mapping_v1','REJECTED','REJECTED','2026-01-01T00:00:00Z');

CREATE TABLE supply_orders (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  provider_order_id TEXT NOT NULL CHECK(provider_order_id<>''),
  first_observed_at TIMESTAMPTZ NOT NULL,
  last_observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(seller_account_id,provider_order_id),
  UNIQUE(id,seller_account_id),
  CHECK(last_observed_at>=first_observed_at)
);
CREATE TABLE supply_order_versions (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  supply_order_id BIGINT NOT NULL,
  version_no BIGINT NOT NULL CHECK(version_no>0),
  provider_state_code TEXT NOT NULL,
  canonical_state TEXT NOT NULL,
  state_mapping_version TEXT NOT NULL,
  state_mapping_status TEXT NOT NULL CHECK(state_mapping_status IN('MAPPED','UNKNOWN')),
  dropoff_location_id BIGINT NULL,
  timeslot_from TIMESTAMPTZ NULL,
  timeslot_to TIMESTAMPTZ NULL,
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  state_hash TEXT NOT NULL CHECK(state_hash~'^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(supply_order_id,version_no),
  UNIQUE(id,seller_account_id),
  FOREIGN KEY(supply_order_id,seller_account_id) REFERENCES supply_orders(id,seller_account_id),
  FOREIGN KEY(dropoff_location_id,seller_account_id) REFERENCES inventory_locations(id,seller_account_id),
  CHECK(timeslot_to IS NULL OR timeslot_from IS NULL OR timeslot_to>=timeslot_from)
);

CREATE TABLE supplies (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  supply_order_id BIGINT NOT NULL,
  provider_supply_id TEXT NOT NULL CHECK(provider_supply_id<>''),
  first_observed_at TIMESTAMPTZ NOT NULL,
  last_observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(seller_account_id,provider_supply_id),
  UNIQUE(id,seller_account_id),
  FOREIGN KEY(supply_order_id,seller_account_id) REFERENCES supply_orders(id,seller_account_id),
  CHECK(last_observed_at>=first_observed_at)
);
CREATE INDEX idx_supplies_order ON supplies(supply_order_id);
CREATE TABLE supply_versions (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  supply_id BIGINT NOT NULL,
  version_no BIGINT NOT NULL CHECK(version_no>0),
  provider_state_code TEXT NOT NULL,
  canonical_state TEXT NOT NULL,
  state_mapping_version TEXT NOT NULL,
  state_mapping_status TEXT NOT NULL CHECK(state_mapping_status IN('MAPPED','UNKNOWN')),
  storage_location_id BIGINT NULL,
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  state_hash TEXT NOT NULL CHECK(state_hash~'^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(supply_id,version_no),
  UNIQUE(id,seller_account_id),
  FOREIGN KEY(supply_id,seller_account_id) REFERENCES supplies(id,seller_account_id),
  FOREIGN KEY(storage_location_id,seller_account_id) REFERENCES inventory_locations(id,seller_account_id)
);

CREATE TABLE supply_bundle_versions (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  supply_id BIGINT NOT NULL,
  version_no BIGINT NOT NULL CHECK(version_no>0),
  provider_bundle_id TEXT NULL,
  completeness_status TEXT NOT NULL CHECK(completeness_status IN('COMPLETE','PARTIAL')),
  page_count BIGINT NOT NULL DEFAULT 0 CHECK(page_count>=0),
  item_count BIGINT NOT NULL DEFAULT 0 CHECK(item_count>=0),
  composition_hash TEXT NOT NULL CHECK(composition_hash~'^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(supply_id,version_no),
  UNIQUE(id,seller_account_id),
  FOREIGN KEY(supply_id,seller_account_id) REFERENCES supplies(id,seller_account_id)
);
CREATE TABLE supply_planned_items (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  supply_bundle_version_id BIGINT NOT NULL,
  line_ordinal BIGINT NOT NULL CHECK(line_ordinal>0),
  canonical_sku_id BIGINT NULL,
  source_sku BIGINT NULL CHECK(source_sku IS NULL OR source_sku>0),
  source_product_id BIGINT NULL CHECK(source_product_id IS NULL OR source_product_id>0),
  source_offer_id TEXT NULL,
  sku_resolution_status TEXT NOT NULL CHECK(sku_resolution_status IN('RESOLVED','UNRESOLVED','CONFLICT')),
  declared_units BIGINT NULL CHECK(declared_units IS NULL OR declared_units>=0),
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(supply_bundle_version_id,line_ordinal),
  FOREIGN KEY(supply_bundle_version_id,seller_account_id) REFERENCES supply_bundle_versions(id,seller_account_id) ON DELETE CASCADE,
  FOREIGN KEY(canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id),
  CHECK((sku_resolution_status='RESOLVED')=(canonical_sku_id IS NOT NULL)),
  CHECK(source_sku IS NOT NULL OR source_product_id IS NOT NULL OR source_offer_id IS NOT NULL)
);

CREATE TABLE supply_state_observations (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  supply_order_id BIGINT NULL,
  supply_id BIGINT NULL,
  provider_state_code TEXT NOT NULL,
  canonical_state TEXT NOT NULL,
  state_mapping_version TEXT NOT NULL,
  state_mapping_status TEXT NOT NULL CHECK(state_mapping_status IN('MAPPED','UNKNOWN')),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  FOREIGN KEY(supply_order_id,seller_account_id) REFERENCES supply_orders(id,seller_account_id),
  FOREIGN KEY(supply_id,seller_account_id) REFERENCES supplies(id,seller_account_id),
  CHECK((supply_order_id IS NOT NULL)<>(supply_id IS NOT NULL))
);

CREATE TABLE supply_acceptance_acts (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  supply_id BIGINT NOT NULL,
  provider_act_id TEXT NOT NULL,
  provider_state TEXT NULL,
  total_amount NUMERIC(30,6) NULL,
  currency_code VARCHAR(3) NULL CHECK(currency_code IS NULL OR currency_code~'^[A-Z]{3}$'),
  completeness_status TEXT NOT NULL CHECK(completeness_status IN('COMPLETE','PARTIAL')),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(seller_account_id,provider_act_id,observed_at),
  UNIQUE(id,seller_account_id),
  FOREIGN KEY(supply_id,seller_account_id) REFERENCES supplies(id,seller_account_id)
);
CREATE TABLE supply_acceptance_items (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  supply_acceptance_act_id BIGINT NOT NULL,
  line_ordinal BIGINT NOT NULL CHECK(line_ordinal>0),
  canonical_sku_id BIGINT NULL,
  source_sku BIGINT NULL,
  source_offer_id TEXT NULL,
  sku_resolution_status TEXT NOT NULL CHECK(sku_resolution_status IN('RESOLVED','UNRESOLVED','CONFLICT')),
  accepted_units BIGINT NULL CHECK(accepted_units IS NULL OR accepted_units>=0),
  shortage_units BIGINT NULL CHECK(shortage_units IS NULL OR shortage_units>=0),
  surplus_units BIGINT NULL CHECK(surplus_units IS NULL OR surplus_units>=0),
  amount NUMERIC(30,6) NULL,
  currency_code VARCHAR(3) NULL CHECK(currency_code IS NULL OR currency_code~'^[A-Z]{3}$'),
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(supply_acceptance_act_id,line_ordinal),
  FOREIGN KEY(supply_acceptance_act_id,seller_account_id) REFERENCES supply_acceptance_acts(id,seller_account_id) ON DELETE CASCADE,
  FOREIGN KEY(canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id),
  CHECK((sku_resolution_status='RESOLVED')=(canonical_sku_id IS NOT NULL))
);

CREATE INDEX idx_supply_order_versions_cutoff ON supply_order_versions(supply_order_id,observed_at DESC,version_no DESC,id DESC);
CREATE INDEX idx_supply_versions_cutoff ON supply_versions(supply_id,observed_at DESC,version_no DESC,id DESC);
CREATE INDEX idx_supply_bundles_cutoff ON supply_bundle_versions(supply_id,observed_at DESC,version_no DESC,id DESC);
CREATE INDEX idx_supply_state_observed ON supply_state_observations(seller_account_id,observed_at DESC);

CREATE OR REPLACE FUNCTION enforce_canonical_supply_scope() RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS(SELECT 1 FROM collection_domain_runs dr WHERE dr.id=NEW.collection_domain_run_id AND dr.seller_account_id=NEW.seller_account_id AND dr.domain='SUPPLY') THEN
    RAISE EXCEPTION 'canonical supply domain-run seller mismatch' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER supply_order_versions_scope BEFORE INSERT OR UPDATE ON supply_order_versions FOR EACH ROW EXECUTE FUNCTION enforce_canonical_supply_scope();
CREATE TRIGGER supply_versions_scope BEFORE INSERT OR UPDATE ON supply_versions FOR EACH ROW EXECUTE FUNCTION enforce_canonical_supply_scope();
CREATE TRIGGER supply_bundles_scope BEFORE INSERT OR UPDATE ON supply_bundle_versions FOR EACH ROW EXECUTE FUNCTION enforce_canonical_supply_scope();
CREATE TRIGGER supply_states_scope BEFORE INSERT OR UPDATE ON supply_state_observations FOR EACH ROW EXECUTE FUNCTION enforce_canonical_supply_scope();
CREATE TRIGGER supply_acts_scope BEFORE INSERT OR UPDATE ON supply_acceptance_acts FOR EACH ROW EXECUTE FUNCTION enforce_canonical_supply_scope();
