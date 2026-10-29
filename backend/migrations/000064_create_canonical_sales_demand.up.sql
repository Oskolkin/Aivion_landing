ALTER TABLE collection_authoritative_heads DROP CONSTRAINT collection_authoritative_heads_domain_check;
ALTER TABLE collection_authoritative_heads ADD CONSTRAINT collection_authoritative_heads_domain_check CHECK (domain IN ('CATALOG','SALES_DEMAND'));

CREATE TABLE sales_demand_daily (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    canonical_sku_id BIGINT NULL REFERENCES canonical_skus(id) ON DELETE RESTRICT,
    source_sku BIGINT NOT NULL CHECK (source_sku > 0),
    business_date DATE NOT NULL,
    ordered_amount NUMERIC(30,6) NOT NULL CHECK (ordered_amount >= 0),
    ordered_units BIGINT NOT NULL CHECK (ordered_units >= 0),
    currency_code VARCHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
    sku_resolution_status TEXT NOT NULL CHECK (sku_resolution_status IN ('RESOLVED','UNRESOLVED')),
    revision_no BIGINT NOT NULL CHECK (revision_no > 0),
    fact_hash TEXT NOT NULL CHECK (fact_hash ~ '^[0-9a-f]{64}$'),
    collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
    source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT sales_demand_daily_revision_unique UNIQUE (seller_account_id,source_sku,business_date,revision_no),
    CONSTRAINT sales_demand_daily_fact_unique UNIQUE (seller_account_id,source_sku,business_date,fact_hash),
    CONSTRAINT sales_demand_daily_resolution_check CHECK ((sku_resolution_status='RESOLVED')=(canonical_sku_id IS NOT NULL))
);
CREATE INDEX idx_sales_demand_daily_current ON sales_demand_daily(seller_account_id,business_date,source_sku,revision_no DESC);
CREATE INDEX idx_sales_demand_daily_canonical ON sales_demand_daily(seller_account_id,canonical_sku_id,business_date) WHERE canonical_sku_id IS NOT NULL;

CREATE TABLE sales_account_daily (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    business_date DATE NOT NULL,
    ordered_amount NUMERIC(30,6) NOT NULL CHECK (ordered_amount >= 0),
    ordered_units BIGINT NOT NULL CHECK (ordered_units >= 0),
    currency_code VARCHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
    revision_no BIGINT NOT NULL CHECK (revision_no > 0),
    fact_hash TEXT NOT NULL CHECK (fact_hash ~ '^[0-9a-f]{64}$'),
    collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
    source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT sales_account_daily_revision_unique UNIQUE (seller_account_id,business_date,revision_no),
    CONSTRAINT sales_account_daily_fact_unique UNIQUE (seller_account_id,business_date,fact_hash)
);
CREATE INDEX idx_sales_account_daily_current ON sales_account_daily(seller_account_id,business_date,revision_no DESC);

CREATE TABLE sales_reconciliations (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    business_date DATE NOT NULL,
    currency_code VARCHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
    provider_account_ordered_amount NUMERIC(30,6) NULL CHECK (provider_account_ordered_amount >= 0),
    provider_account_ordered_units BIGINT NULL CHECK (provider_account_ordered_units >= 0),
    source_sku_ordered_amount NUMERIC(30,6) NOT NULL CHECK (source_sku_ordered_amount >= 0),
    source_sku_ordered_units BIGINT NOT NULL CHECK (source_sku_ordered_units >= 0),
    resolved_sku_ordered_amount NUMERIC(30,6) NOT NULL CHECK (resolved_sku_ordered_amount >= 0),
    resolved_sku_ordered_units BIGINT NOT NULL CHECK (resolved_sku_ordered_units >= 0),
    unresolved_ordered_amount NUMERIC(30,6) NOT NULL CHECK (unresolved_ordered_amount >= 0),
    unresolved_ordered_units BIGINT NOT NULL CHECK (unresolved_ordered_units >= 0),
    source_sku_row_count BIGINT NOT NULL CHECK (source_sku_row_count >= 0),
    resolved_sku_row_count BIGINT NOT NULL CHECK (resolved_sku_row_count >= 0),
    unresolved_sku_row_count BIGINT NOT NULL CHECK (unresolved_sku_row_count >= 0),
    amount_gap_account_vs_all_source NUMERIC(30,6) NULL,
    units_gap_account_vs_all_source BIGINT NULL,
    amount_gap_account_vs_resolved NUMERIC(30,6) NULL,
    units_gap_account_vs_resolved BIGINT NULL,
    amount_status TEXT NOT NULL CHECK (amount_status IN ('PASS','EXPLAINED_GAP','UNEXPLAINED_GAP','NOT_COMPARABLE')),
    units_status TEXT NOT NULL CHECK (units_status IN ('PASS','EXPLAINED_GAP','UNEXPLAINED_GAP','NOT_COMPARABLE')),
    overall_status TEXT NOT NULL CHECK (overall_status IN ('PASS','EXPLAINED_GAP','UNEXPLAINED_GAP','NOT_COMPARABLE')),
    collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
    observed_at TIMESTAMPTZ NOT NULL,
    details_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT sales_reconciliations_run_day_unique UNIQUE (collection_domain_run_id,business_date)
);
CREATE INDEX idx_sales_reconciliations_seller_date ON sales_reconciliations(seller_account_id,business_date DESC);

CREATE OR REPLACE FUNCTION enforce_sales_demand_scope() RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM collection_domain_runs dr WHERE dr.id=NEW.collection_domain_run_id AND dr.seller_account_id=NEW.seller_account_id AND dr.domain='SALES_DEMAND') THEN
    RAISE EXCEPTION 'sales fact domain run seller/domain mismatch' USING ERRCODE='23514';
  END IF;
  IF TG_TABLE_NAME IN ('sales_demand_daily','sales_account_daily') THEN
    IF NOT EXISTS (SELECT 1 FROM source_payloads sp WHERE sp.id=NEW.source_payload_id AND sp.seller_account_id=NEW.seller_account_id AND sp.collection_domain_run_id=NEW.collection_domain_run_id) THEN
      RAISE EXCEPTION 'sales fact payload seller/domain-run mismatch' USING ERRCODE='23514';
    END IF;
  END IF;
  IF TG_TABLE_NAME='sales_demand_daily' THEN
    IF NEW.canonical_sku_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM canonical_skus cs WHERE cs.id=NEW.canonical_sku_id AND cs.seller_account_id=NEW.seller_account_id AND cs.ozon_sku=NEW.source_sku) THEN
      RAISE EXCEPTION 'sales fact canonical SKU seller/source mismatch' USING ERRCODE='23514';
    END IF;
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;

CREATE TRIGGER sales_demand_daily_scope_trigger BEFORE INSERT OR UPDATE ON sales_demand_daily FOR EACH ROW EXECUTE FUNCTION enforce_sales_demand_scope();
CREATE TRIGGER sales_account_daily_scope_trigger BEFORE INSERT OR UPDATE ON sales_account_daily FOR EACH ROW EXECUTE FUNCTION enforce_sales_demand_scope();
CREATE TRIGGER sales_reconciliations_scope_trigger BEFORE INSERT OR UPDATE ON sales_reconciliations FOR EACH ROW EXECUTE FUNCTION enforce_sales_demand_scope();
