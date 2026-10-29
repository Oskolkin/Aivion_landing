-- Prompt №8.2: append-only historical domain watermark revisions (AS KNOWN AT).
-- domain_watermarks remains the mutable latest projection for operational reads.
-- RESOLVE_CONTEXT must read domain_watermark_revisions, never only the projection.

CREATE TABLE domain_watermark_revisions (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    domain TEXT NOT NULL CHECK (domain IN (
        'CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING',
        'FUNNEL','FINANCE','SEARCH','PROMOTIONS','ACCOUNT_HEALTH','SUPPLY'
    )),
    fact_family TEXT NOT NULL,
    latest_observed_at TIMESTAMPTZ NULL,
    latest_complete_business_date DATE NULL,
    latest_mature_business_date DATE NULL,
    latest_complete_snapshot_at TIMESTAMPTZ NULL,
    history_available_from DATE NULL,
    history_available_to DATE NULL,
    last_successful_domain_run_id BIGINT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
    source_contract_hash TEXT NULL,
    state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
    assessed_at TIMESTAMPTZ NOT NULL,
    knowledge_time TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        history_available_to IS NULL
        OR history_available_from IS NULL
        OR history_available_to >= history_available_from
    )
);

CREATE INDEX idx_domain_watermark_revisions_as_known
    ON domain_watermark_revisions (
        seller_account_id,
        domain,
        fact_family,
        knowledge_time DESC,
        id DESC
    );

-- Seed one revision per current projection row. Do not invent older history.
INSERT INTO domain_watermark_revisions (
    seller_account_id,
    domain,
    fact_family,
    latest_observed_at,
    latest_complete_business_date,
    latest_mature_business_date,
    latest_complete_snapshot_at,
    history_available_from,
    history_available_to,
    last_successful_domain_run_id,
    source_contract_hash,
    state_hash,
    assessed_at,
    knowledge_time
)
SELECT
    seller_account_id,
    domain,
    fact_family,
    latest_observed_at,
    latest_complete_business_date,
    latest_mature_business_date,
    latest_complete_snapshot_at,
    history_available_from,
    history_available_to,
    last_successful_domain_run_id,
    source_contract_hash,
    state_hash,
    assessed_at,
    assessed_at
FROM domain_watermarks;
