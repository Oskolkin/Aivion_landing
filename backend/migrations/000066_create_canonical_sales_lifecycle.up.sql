ALTER TABLE collection_authoritative_heads DROP CONSTRAINT collection_authoritative_heads_domain_check;
ALTER TABLE collection_authoritative_heads ADD CONSTRAINT collection_authoritative_heads_domain_check
    CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS'));

ALTER TABLE canonical_skus ADD CONSTRAINT canonical_skus_id_seller_unique UNIQUE (id,seller_account_id);

CREATE TABLE sales_orders (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    ozon_order_id BIGINT NOT NULL CHECK (ozon_order_id > 0),
    order_number TEXT NULL,
    first_observed_at TIMESTAMPTZ NOT NULL,
    last_observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_account_id,ozon_order_id),
    UNIQUE (id,seller_account_id),
    CHECK (last_observed_at >= first_observed_at)
);

CREATE TABLE sales_postings (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    canonical_order_id BIGINT NULL,
    posting_number TEXT NOT NULL CHECK (btrim(posting_number) <> ''),
    fulfillment_scheme TEXT NOT NULL CHECK (fulfillment_scheme IN ('FBO','FBS','RFBS','UNKNOWN')),
    order_resolution_status TEXT NOT NULL CHECK (order_resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT')),
    first_observed_at TIMESTAMPTZ NOT NULL,
    last_observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_account_id,posting_number),
    UNIQUE (id,seller_account_id),
    FOREIGN KEY (canonical_order_id,seller_account_id) REFERENCES sales_orders(id,seller_account_id) ON DELETE RESTRICT,
    CHECK ((order_resolution_status='RESOLVED')=(canonical_order_id IS NOT NULL)),
    CHECK (last_observed_at >= first_observed_at)
);

CREATE TABLE sales_posting_versions (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    canonical_posting_id BIGINT NOT NULL,
    version_no BIGINT NOT NULL CHECK (version_no > 0),
    collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
    source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
    source_received_at TIMESTAMPTZ NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    provider_status TEXT NOT NULL,
    provider_substatus TEXT NULL,
    provider_created_at TIMESTAMPTZ NULL,
    in_process_at TIMESTAMPTZ NULL,
    delivering_date DATE NULL,
    shipment_date DATE NULL,
    cancel_reason_id BIGINT NULL,
    cancel_reason TEXT NULL,
    cancellation_type TEXT NULL,
    cancellation_initiator TEXT NULL,
    cancelled_after_ship BOOLEAN NULL,
    warehouse_id BIGINT NULL,
    provider_id BIGINT NULL,
    state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (canonical_posting_id,version_no),
    UNIQUE (id,seller_account_id),
    FOREIGN KEY (canonical_posting_id,seller_account_id) REFERENCES sales_postings(id,seller_account_id) ON DELETE RESTRICT
);
CREATE INDEX idx_sales_posting_versions_latest ON sales_posting_versions(canonical_posting_id,version_no DESC);

CREATE TABLE sales_posting_products (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    posting_version_id BIGINT NOT NULL,
    line_no INTEGER NOT NULL CHECK (line_no > 0),
    canonical_sku_id BIGINT NULL,
    source_sku BIGINT NOT NULL CHECK (source_sku > 0),
    sku_resolution_status TEXT NOT NULL CHECK (sku_resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT')),
    offer_id TEXT NULL,
    quantity BIGINT NOT NULL CHECK (quantity >= 0),
    unit_price NUMERIC(30,6) NOT NULL CHECK (unit_price >= 0),
    currency_code VARCHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
    product_name TEXT NULL,
    source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (posting_version_id,line_no),
    FOREIGN KEY (posting_version_id,seller_account_id) REFERENCES sales_posting_versions(id,seller_account_id) ON DELETE CASCADE,
    FOREIGN KEY (canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id) ON DELETE RESTRICT,
    CHECK ((sku_resolution_status='RESOLVED')=(canonical_sku_id IS NOT NULL))
);

CREATE TABLE sales_posting_relations (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    parent_posting_id BIGINT NULL,
    parent_posting_number TEXT NOT NULL CHECK (btrim(parent_posting_number) <> ''),
    child_posting_id BIGINT NOT NULL,
    relation_type TEXT NOT NULL CHECK (relation_type IN ('SPLIT')),
    resolution_status TEXT NOT NULL CHECK (resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT')),
    first_observed_at TIMESTAMPTZ NOT NULL,
    source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
    collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_account_id,parent_posting_number,child_posting_id,relation_type),
    FOREIGN KEY (parent_posting_id,seller_account_id) REFERENCES sales_postings(id,seller_account_id) ON DELETE RESTRICT,
    FOREIGN KEY (child_posting_id,seller_account_id) REFERENCES sales_postings(id,seller_account_id) ON DELETE RESTRICT,
    CHECK (parent_posting_id IS NULL OR parent_posting_id <> child_posting_id),
    CHECK ((resolution_status='RESOLVED')=(parent_posting_id IS NOT NULL))
);

CREATE TABLE sales_returns (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    provider_return_id BIGINT NOT NULL CHECK (provider_return_id > 0),
    first_observed_at TIMESTAMPTZ NOT NULL,
    last_observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_account_id,provider_return_id),
    UNIQUE (id,seller_account_id),
    CHECK (last_observed_at >= first_observed_at)
);

CREATE TABLE sales_return_events (
    id BIGSERIAL PRIMARY KEY,
    sales_return_id BIGINT NOT NULL,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    version_no BIGINT NOT NULL CHECK (version_no > 0),
    canonical_order_id BIGINT NULL,
    canonical_posting_id BIGINT NULL,
    canonical_sku_id BIGINT NULL,
    source_order_id BIGINT NULL,
    source_posting_number TEXT NULL,
    source_sku BIGINT NULL,
    order_resolution_status TEXT NOT NULL CHECK (order_resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT','NOT_APPLICABLE')),
    posting_resolution_status TEXT NOT NULL CHECK (posting_resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT','NOT_APPLICABLE')),
    sku_resolution_status TEXT NOT NULL CHECK (sku_resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT','NOT_APPLICABLE')),
    provider_type TEXT NULL,
    provider_schema TEXT NULL,
    return_reason_name TEXT NULL,
    provider_source_id BIGINT NULL,
    quantity BIGINT NOT NULL CHECK (quantity >= 0),
    unit_price NUMERIC(30,6) NOT NULL CHECK (unit_price >= 0),
    currency_code VARCHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
    provider_return_date TIMESTAMPTZ NULL,
    provider_visual_status TEXT NULL,
    provider_visual_status_sys_name TEXT NULL,
    provider_visual_change_at TIMESTAMPTZ NULL,
    technical_return_moment TIMESTAMPTZ NULL,
    final_moment TIMESTAMPTZ NULL,
    cancelled_with_compensation_moment TIMESTAMPTZ NULL,
    compensation_status TEXT NULL,
    state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
    source_received_at TIMESTAMPTZ NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
    collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (sales_return_id,version_no),
    UNIQUE (id,seller_account_id),
    FOREIGN KEY (sales_return_id,seller_account_id) REFERENCES sales_returns(id,seller_account_id) ON DELETE RESTRICT,
    FOREIGN KEY (canonical_order_id,seller_account_id) REFERENCES sales_orders(id,seller_account_id) ON DELETE RESTRICT,
    FOREIGN KEY (canonical_posting_id,seller_account_id) REFERENCES sales_postings(id,seller_account_id) ON DELETE RESTRICT,
    FOREIGN KEY (canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id) ON DELETE RESTRICT
);
CREATE INDEX idx_sales_return_events_latest ON sales_return_events(sales_return_id,version_no DESC);

CREATE TABLE sales_lifecycle_events (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    canonical_order_id BIGINT NULL,
    canonical_posting_id BIGINT NULL,
    canonical_sku_id BIGINT NULL,
    canonical_return_id BIGINT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('POSTING_CREATED','POSTING_SPLIT','POSTING_DELIVERING','POSTING_DELIVERED','POSTING_CANCELLED_FULL','POSTING_CANCELLED_PARTIAL','RETURN_CREATED')),
    quantity BIGINT NULL CHECK (quantity IS NULL OR quantity >= 0),
    unit_amount NUMERIC(30,6) NULL CHECK (unit_amount IS NULL OR unit_amount >= 0),
    amount NUMERIC(30,6) NULL CHECK (amount IS NULL OR amount >= 0),
    currency_code VARCHAR(3) NULL CHECK (currency_code IS NULL OR currency_code ~ '^[A-Z]{3}$'),
    effective_at TIMESTAMPTZ NULL,
    provider_date DATE NULL,
    effective_window_from_at TIMESTAMPTZ NULL,
    effective_window_to_at TIMESTAMPTZ NULL,
    business_date DATE NULL,
    time_quality TEXT NOT NULL CHECK (time_quality IN ('PROVIDER_EXACT','PROVIDER_DATE','OBSERVED_APPROX','UNKNOWN')),
    source_type TEXT NOT NULL CHECK (source_type IN ('POSTING_VERSION','POSTING_RELATION','RETURN_OBSERVATION')),
    source_event_id TEXT NULL,
    source_posting_version_id BIGINT NULL,
    source_return_event_id BIGINT NULL,
    source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
    collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
    dedupe_key TEXT NOT NULL CHECK (dedupe_key ~ '^[0-9a-f]{64}$'),
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_account_id,dedupe_key),
    FOREIGN KEY (canonical_order_id,seller_account_id) REFERENCES sales_orders(id,seller_account_id) ON DELETE RESTRICT,
    FOREIGN KEY (canonical_posting_id,seller_account_id) REFERENCES sales_postings(id,seller_account_id) ON DELETE RESTRICT,
    FOREIGN KEY (canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id) ON DELETE RESTRICT,
    FOREIGN KEY (canonical_return_id,seller_account_id) REFERENCES sales_returns(id,seller_account_id) ON DELETE RESTRICT,
    FOREIGN KEY (source_posting_version_id,seller_account_id) REFERENCES sales_posting_versions(id,seller_account_id) ON DELETE RESTRICT,
    FOREIGN KEY (source_return_event_id,seller_account_id) REFERENCES sales_return_events(id,seller_account_id) ON DELETE RESTRICT,
    CHECK (effective_window_to_at IS NULL OR effective_window_from_at IS NULL OR effective_window_to_at >= effective_window_from_at),
    CHECK ((time_quality='PROVIDER_EXACT')=(effective_at IS NOT NULL) OR time_quality <> 'PROVIDER_EXACT'),
    CHECK (time_quality <> 'OBSERVED_APPROX' OR effective_at IS NULL)
);
CREATE INDEX idx_sales_lifecycle_events_seller_observed ON sales_lifecycle_events(seller_account_id,observed_at DESC);

CREATE OR REPLACE FUNCTION enforce_sales_lifecycle_domain_scope() RETURNS TRIGGER AS $$
DECLARE expected_domain TEXT;
BEGIN
  expected_domain := CASE
    WHEN TG_TABLE_NAME='sales_return_events' THEN 'RETURNS'
    WHEN TG_TABLE_NAME='sales_lifecycle_events' AND to_jsonb(NEW)->>'source_type'='RETURN_OBSERVATION' THEN 'RETURNS'
    ELSE 'POSTINGS'
  END;
  IF TG_TABLE_NAME NOT IN ('sales_orders','sales_postings','sales_returns') AND NOT EXISTS (
    SELECT 1 FROM collection_domain_runs dr WHERE dr.id=NEW.collection_domain_run_id AND dr.seller_account_id=NEW.seller_account_id AND dr.domain=expected_domain
  ) THEN RAISE EXCEPTION 'canonical lifecycle domain run seller/domain mismatch' USING ERRCODE='23514'; END IF;
  IF TG_TABLE_NAME NOT IN ('sales_orders','sales_postings','sales_returns') AND NOT EXISTS (
    SELECT 1 FROM source_payloads sp WHERE sp.id=NEW.source_payload_id AND sp.seller_account_id=NEW.seller_account_id AND sp.collection_domain_run_id=NEW.collection_domain_run_id
  ) THEN RAISE EXCEPTION 'canonical lifecycle payload seller/domain-run mismatch' USING ERRCODE='23514'; END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION enforce_sales_posting_product_scope() RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM source_payloads sp
    JOIN sales_posting_versions pv ON pv.id=NEW.posting_version_id
    JOIN collection_domain_runs dr ON dr.id=pv.collection_domain_run_id
    WHERE sp.id=NEW.source_payload_id
      AND sp.seller_account_id=NEW.seller_account_id
      AND sp.collection_domain_run_id=pv.collection_domain_run_id
      AND pv.seller_account_id=NEW.seller_account_id
      AND dr.seller_account_id=NEW.seller_account_id
      AND dr.domain='POSTINGS'
  ) THEN
    RAISE EXCEPTION 'posting product payload seller/domain-run mismatch' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;

CREATE TRIGGER sales_posting_versions_scope BEFORE INSERT OR UPDATE ON sales_posting_versions FOR EACH ROW EXECUTE FUNCTION enforce_sales_lifecycle_domain_scope();
CREATE TRIGGER sales_posting_relations_scope BEFORE INSERT OR UPDATE ON sales_posting_relations FOR EACH ROW EXECUTE FUNCTION enforce_sales_lifecycle_domain_scope();
CREATE TRIGGER sales_posting_products_scope BEFORE INSERT OR UPDATE ON sales_posting_products FOR EACH ROW EXECUTE FUNCTION enforce_sales_posting_product_scope();
CREATE TRIGGER sales_return_events_scope BEFORE INSERT OR UPDATE ON sales_return_events FOR EACH ROW EXECUTE FUNCTION enforce_sales_lifecycle_domain_scope();
CREATE TRIGGER sales_lifecycle_events_scope BEFORE INSERT OR UPDATE ON sales_lifecycle_events FOR EACH ROW EXECUTE FUNCTION enforce_sales_lifecycle_domain_scope();
