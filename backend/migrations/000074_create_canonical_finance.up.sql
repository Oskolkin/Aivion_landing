CREATE TABLE finance_provider_types (
  id BIGSERIAL PRIMARY KEY,
  provider TEXT NOT NULL CHECK (btrim(provider) <> ''),
  provider_type_id TEXT NOT NULL CHECK (btrim(provider_type_id) <> ''),
  first_observed_at TIMESTAMPTZ NOT NULL,
  last_observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (provider, provider_type_id),
  CHECK (last_observed_at >= first_observed_at)
);

CREATE TABLE finance_provider_type_versions (
  id BIGSERIAL PRIMARY KEY,
  finance_provider_type_id BIGINT NOT NULL REFERENCES finance_provider_types(id) ON DELETE CASCADE,
  version_no BIGINT NOT NULL CHECK (version_no > 0),
  name TEXT NOT NULL,
  description TEXT NULL,
  canonical_group TEXT NULL,
  canonical_subtype TEXT NULL,
  economic_role TEXT NULL,
  default_scope TEXT NOT NULL CHECK (default_scope IN ('ACCOUNT','POSTING','ORDER','SKU','UNKNOWN')),
  cross_domain_owner TEXT NULL,
  mapping_status TEXT NOT NULL CHECK (mapping_status IN ('MAPPED','UNMAPPED','PENDING_ACCEPTANCE')),
  source_dictionary_hash TEXT NOT NULL CHECK (source_dictionary_hash ~ '^[0-9a-f]{64}$'),
  valid_from TIMESTAMPTZ NOT NULL,
  valid_to TIMESTAMPTZ NULL,
  observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (finance_provider_type_id, version_no),
  CHECK (valid_to IS NULL OR valid_to >= valid_from)
);
CREATE UNIQUE INDEX finance_provider_type_versions_open
  ON finance_provider_type_versions(finance_provider_type_id) WHERE valid_to IS NULL;

CREATE TABLE finance_accrual_day_revisions (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  business_date DATE NOT NULL,
  revision_no BIGINT NOT NULL CHECK (revision_no > 0),
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  completeness_status TEXT NOT NULL CHECK (completeness_status IN ('COMPLETE','PARTIAL')),
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (id, seller_account_id),
  UNIQUE (seller_account_id, business_date, revision_no),
  UNIQUE (seller_account_id, business_date, state_hash)
);
CREATE INDEX idx_finance_day_cutoff ON finance_accrual_day_revisions
  (seller_account_id, business_date, observed_at DESC, revision_no DESC, id DESC);

CREATE TABLE finance_accrual_records (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  day_revision_id BIGINT NOT NULL,
  row_ordinal INTEGER NOT NULL CHECK (row_ordinal > 0),
  total_amount_signed NUMERIC(30,6) NOT NULL,
  currency_code VARCHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
  finance_provider_type_id BIGINT NULL REFERENCES finance_provider_types(id) ON DELETE RESTRICT,
  provider_type_id TEXT NOT NULL CHECK (btrim(provider_type_id) <> ''),
  provider_accrued_category TEXT NULL,
  unit_number TEXT NULL,
  source_fragment_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  FOREIGN KEY (day_revision_id, seller_account_id)
    REFERENCES finance_accrual_day_revisions(id, seller_account_id) ON DELETE CASCADE,
  UNIQUE (day_revision_id, row_ordinal)
);
CREATE INDEX idx_finance_records_type ON finance_accrual_records
  (seller_account_id, finance_provider_type_id, day_revision_id);

CREATE TABLE finance_accrual_lines (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  finance_accrual_record_id BIGINT NOT NULL REFERENCES finance_accrual_records(id) ON DELETE CASCADE,
  line_ordinal INTEGER NOT NULL CHECK (line_ordinal > 0),
  source_component_path TEXT NOT NULL CHECK (btrim(source_component_path) <> ''),
  signed_amount NUMERIC(30,6) NOT NULL,
  mapping_status TEXT NOT NULL CHECK (mapping_status IN ('MAPPED','UNMAPPED','PENDING_ACCEPTANCE')),
  canonical_sku_id BIGINT NULL,
  canonical_order_id BIGINT NULL,
  canonical_posting_id BIGINT NULL,
  source_ozon_sku BIGINT NULL CHECK (source_ozon_sku IS NULL OR source_ozon_sku > 0),
  source_product_id BIGINT NULL CHECK (source_product_id IS NULL OR source_product_id > 0),
  source_fragment_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  FOREIGN KEY (canonical_sku_id, seller_account_id) REFERENCES canonical_skus(id, seller_account_id),
  FOREIGN KEY (canonical_order_id, seller_account_id) REFERENCES sales_orders(id, seller_account_id),
  FOREIGN KEY (canonical_posting_id, seller_account_id) REFERENCES sales_postings(id, seller_account_id),
  UNIQUE (finance_accrual_record_id, line_ordinal)
);

CREATE TABLE finance_balance_snapshots (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  balance_amount_signed NUMERIC(30,6) NOT NULL,
  currency_code VARCHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
  completeness_status TEXT NOT NULL CHECK (completeness_status IN ('COMPLETE','PARTIAL')),
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_finance_balance_cutoff ON finance_balance_snapshots
  (seller_account_id, observed_at DESC, id DESC);

CREATE OR REPLACE FUNCTION enforce_canonical_finance_scope() RETURNS TRIGGER AS $$
DECLARE
  run_id BIGINT;
  payload_id BIGINT;
BEGIN
  IF TG_TABLE_NAME = 'finance_accrual_records' THEN
    SELECT collection_domain_run_id, source_payload_id INTO run_id, payload_id
    FROM finance_accrual_day_revisions
    WHERE id = NEW.day_revision_id AND seller_account_id = NEW.seller_account_id;
  ELSIF TG_TABLE_NAME = 'finance_accrual_lines' THEN
    SELECT d.collection_domain_run_id, d.source_payload_id INTO run_id, payload_id
    FROM finance_accrual_records r
    JOIN finance_accrual_day_revisions d ON d.id = r.day_revision_id
    WHERE r.id = NEW.finance_accrual_record_id
      AND r.seller_account_id = NEW.seller_account_id;
  ELSE
    run_id := NEW.collection_domain_run_id;
    payload_id := NEW.source_payload_id;
  END IF;
  IF run_id IS NULL OR NOT EXISTS (
    SELECT 1 FROM collection_domain_runs dr
    WHERE dr.id = run_id AND dr.seller_account_id = NEW.seller_account_id
      AND dr.domain = 'FINANCE'
  ) THEN
    RAISE EXCEPTION 'canonical finance domain-run seller mismatch' USING ERRCODE='23514';
  END IF;
  IF payload_id IS NULL OR NOT EXISTS (
    SELECT 1 FROM source_payloads sp
    WHERE sp.id = payload_id AND sp.seller_account_id = NEW.seller_account_id
      AND sp.collection_domain_run_id = run_id
  ) THEN
    RAISE EXCEPTION 'canonical finance payload seller/domain-run mismatch' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;

CREATE TRIGGER finance_day_scope BEFORE INSERT OR UPDATE ON finance_accrual_day_revisions
  FOR EACH ROW EXECUTE FUNCTION enforce_canonical_finance_scope();
CREATE TRIGGER finance_record_scope BEFORE INSERT OR UPDATE ON finance_accrual_records
  FOR EACH ROW EXECUTE FUNCTION enforce_canonical_finance_scope();
CREATE TRIGGER finance_line_scope BEFORE INSERT OR UPDATE ON finance_accrual_lines
  FOR EACH ROW EXECUTE FUNCTION enforce_canonical_finance_scope();
CREATE TRIGGER finance_balance_scope BEFORE INSERT OR UPDATE ON finance_balance_snapshots
  FOR EACH ROW EXECUTE FUNCTION enforce_canonical_finance_scope();
