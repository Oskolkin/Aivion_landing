CREATE TABLE collection_runs (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    trigger TEXT NULL,
    trigger_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    requested_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT collection_runs_kind_check CHECK (kind IN ('INITIAL', 'DAILY', 'MANUAL', 'REPAIR')),
    CONSTRAINT collection_runs_status_check CHECK (status IN ('PENDING', 'RUNNING', 'COMPLETED', 'COMPLETED_WITH_LIMITATIONS', 'FAILED', 'CANCELLED'))
);

CREATE INDEX idx_collection_runs_seller_status_created_at ON collection_runs(seller_account_id, status, created_at DESC);

CREATE TABLE collection_domain_runs (
    id BIGSERIAL PRIMARY KEY,
    collection_run_id BIGINT NOT NULL REFERENCES collection_runs(id) ON DELETE CASCADE,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    capability_key TEXT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    requested_from DATE NULL,
    requested_to DATE NULL,
    coverage_from DATE NULL,
    coverage_to DATE NULL,
    snapshot_at TIMESTAMPTZ NULL,
    records_received BIGINT NOT NULL DEFAULT 0,
    records_written BIGINT NOT NULL DEFAULT 0,
    pages_received BIGINT NOT NULL DEFAULT 0,
    completeness_status TEXT NULL,
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    source_revision_hash TEXT NULL,
    error_code TEXT NULL,
    error_message TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT collection_domain_runs_domain_check CHECK (domain IN ('CATALOG', 'SALES_DEMAND', 'POSTINGS', 'RETURNS', 'STOCK', 'PRICE', 'ADVERTISING', 'FUNNEL', 'FINANCE', 'SEARCH', 'PROMOTIONS', 'ACCOUNT_HEALTH', 'SUPPLY', 'TAXONOMY')),
    CONSTRAINT collection_domain_runs_status_check CHECK (status IN ('PENDING', 'RUNNING', 'COMPLETED', 'PARTIAL', 'FAILED', 'CANCELLED', 'UNAVAILABLE', 'NOT_APPLICABLE')),
    CONSTRAINT collection_domain_runs_completeness_check CHECK (completeness_status IS NULL OR completeness_status IN ('COMPLETE', 'PARTIAL', 'UNKNOWN', 'UNAVAILABLE', 'NOT_APPLICABLE')),
    CONSTRAINT collection_domain_runs_counters_check CHECK (records_received >= 0 AND records_written >= 0 AND pages_received >= 0)
);

CREATE INDEX idx_collection_domain_runs_collection_run_id ON collection_domain_runs(collection_run_id, id);
CREATE INDEX idx_collection_domain_runs_seller_status_created_at ON collection_domain_runs(seller_account_id, status, created_at DESC);

CREATE OR REPLACE FUNCTION enforce_collection_domain_run_seller_match()
RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM collection_runs cr
        WHERE cr.id = NEW.collection_run_id AND cr.seller_account_id = NEW.seller_account_id
    ) THEN
        RAISE EXCEPTION 'collection_domain_runs seller_account_id must match collection_runs seller_account_id'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER collection_domain_runs_seller_match_trigger
BEFORE INSERT OR UPDATE OF collection_run_id, seller_account_id ON collection_domain_runs
FOR EACH ROW EXECUTE FUNCTION enforce_collection_domain_run_seller_match();

CREATE TABLE collection_cursors (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    cursor_key TEXT NOT NULL,
    cursor_value JSONB NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT collection_cursors_domain_check CHECK (domain IN ('CATALOG', 'SALES_DEMAND', 'POSTINGS', 'RETURNS', 'STOCK', 'PRICE', 'ADVERTISING', 'FUNNEL', 'FINANCE', 'SEARCH', 'PROMOTIONS', 'ACCOUNT_HEALTH', 'SUPPLY', 'TAXONOMY')),
    CONSTRAINT collection_cursors_identity_unique UNIQUE (seller_account_id, domain, cursor_key)
);

CREATE TABLE source_payloads (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    endpoint_key TEXT NOT NULL,
    request_fingerprint TEXT NOT NULL,
    page_seq BIGINT NULL,
    storage_bucket TEXT NOT NULL,
    storage_object_key TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    http_status INTEGER NULL,
    received_at TIMESTAMPTZ NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT source_payloads_provider_check CHECK (provider IN ('OZON')),
    CONSTRAINT source_payloads_page_seq_check CHECK (page_seq IS NULL OR page_seq >= 0),
    CONSTRAINT source_payloads_http_status_check CHECK (http_status IS NULL OR (http_status >= 100 AND http_status <= 599)),
    CONSTRAINT source_payloads_sha256_check CHECK (sha256 ~ '^[0-9a-f]{64}$')
);

CREATE INDEX idx_source_payloads_seller_received_at ON source_payloads(seller_account_id, received_at DESC);
CREATE INDEX idx_source_payloads_domain_run_id ON source_payloads(collection_domain_run_id, id);

CREATE OR REPLACE FUNCTION enforce_source_payload_seller_match()
RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM collection_domain_runs cdr
        WHERE cdr.id = NEW.collection_domain_run_id AND cdr.seller_account_id = NEW.seller_account_id
    ) THEN
        RAISE EXCEPTION 'source_payloads seller_account_id must match collection_domain_runs seller_account_id'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER source_payloads_seller_match_trigger
BEFORE INSERT OR UPDATE OF collection_domain_run_id, seller_account_id ON source_payloads
FOR EACH ROW EXECUTE FUNCTION enforce_source_payload_seller_match();
