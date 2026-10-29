ALTER TABLE sales_reconciliations
    ADD COLUMN normalized_day_hash TEXT NULL,
    ADD CONSTRAINT sales_reconciliations_day_hash_check
        CHECK (normalized_day_hash IS NULL OR normalized_day_hash ~ '^[0-9a-f]{64}$');

CREATE TABLE data_quality_runs (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    cutoff_at TIMESTAMPTZ NOT NULL,
    policy_version TEXT NOT NULL,
    input_hash TEXT NOT NULL CHECK (input_hash ~ '^[0-9a-f]{64}$'),
    result_hash TEXT NULL CHECK (result_hash IS NULL OR result_hash ~ '^[0-9a-f]{64}$'),
    status TEXT NOT NULL CHECK (status IN ('RUNNING','COMPLETED','FAILED')),
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_account_id,cutoff_at,policy_version,input_hash)
);

CREATE TABLE data_quality_issues (
    id BIGSERIAL PRIMARY KEY,
    data_quality_run_id BIGINT NOT NULL REFERENCES data_quality_runs(id) ON DELETE CASCADE,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    domain TEXT NOT NULL CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS')),
    dimension TEXT NOT NULL CHECK (dimension IN ('SCHEMA','COMPLETENESS','UNIQUENESS','INTEGRITY','VALIDITY','SEMANTIC','RESOLUTION','RECONCILIATION','TEMPORAL','FRESHNESS_MATURITY')),
    code TEXT NOT NULL,
    scope_kind TEXT NULL,
    scope_ref TEXT NULL,
    period_start DATE NULL,
    period_end DATE NULL,
    severity TEXT NOT NULL CHECK (severity IN ('INFO','WARNING','ERROR','CRITICAL')),
    readiness_effect TEXT NOT NULL CHECK (readiness_effect IN ('NONE','LIMITATION_ONLY','DEGRADE_TO_PARTIAL','DEGRADE_TO_STALE','FAIL_CAPABILITY','FAIL_DOMAIN','BLOCK_ANALYSIS_DATE')),
    retry_class TEXT NOT NULL CHECK (retry_class IN ('TRANSIENT','STRUCTURAL','DATA_STATE')),
    details_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    detected_at TIMESTAMPTZ NOT NULL,
    CHECK (period_end IS NULL OR period_start IS NULL OR period_end >= period_start)
);

CREATE TABLE business_capability_assessments (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    data_quality_run_id BIGINT NOT NULL REFERENCES data_quality_runs(id) ON DELETE CASCADE,
    capability_key TEXT NOT NULL,
    capability_scope TEXT NOT NULL CHECK (capability_scope IN ('CURRENT_COLLECTION','HISTORICAL_QUERY')),
    domain TEXT NOT NULL CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS')),
    final_state TEXT NOT NULL CHECK (final_state IN ('READY','PARTIAL','STALE','UNAVAILABLE','FAILED','NOT_APPLICABLE')),
    contract_state TEXT NOT NULL CHECK (contract_state IN ('ACTIVE','BETA','DEPRECATED','REMOVED','UNKNOWN')),
    access_state TEXT NOT NULL CHECK (access_state IN ('ACCESSIBLE','UNAVAILABLE_SUBSCRIPTION','UNAVAILABLE_SCOPE','AUTH_FAILED','PROVIDER_FORBIDDEN','UNKNOWN')),
    source_state TEXT NOT NULL CHECK (source_state IN ('SUCCESS','EMPTY_VALID','PARTIAL','FAILED','NOT_ATTEMPTED')),
    semantic_state TEXT NOT NULL CHECK (semantic_state IN ('VALIDATED','PENDING_ACCEPTANCE','PARTIAL_MAPPING','UNKNOWN_PROVIDER_SEMANTICS','INCOMPATIBLE','INVALID')),
    coverage_state TEXT NOT NULL CHECK (coverage_state IN ('FULL','PARTIAL','SPARSE_SOURCE','ACCOUNT_ONLY','DIRECT_SUBSET','UNKNOWN')),
    temporal_state TEXT NOT NULL CHECK (temporal_state IN ('CURRENT','LAGGED','STALE','PERIOD_PARTIAL','OUTSIDE_ANALYSIS_CUTOFF','UNKNOWN')),
    history_state TEXT NOT NULL CHECK (history_state IN ('AVAILABLE','PARTIAL','INSUFFICIENT','NONE','UNKNOWN')),
    population_state TEXT NOT NULL CHECK (population_state IN ('NON_EMPTY','EMPTY','UNKNOWN')),
    reason_code TEXT NULL,
    details_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    policy_version TEXT NOT NULL,
    assessed_at TIMESTAMPTZ NOT NULL,
    source_domain_run_id BIGINT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (data_quality_run_id,capability_key,capability_scope)
);

CREATE TABLE domain_watermarks (
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    domain TEXT NOT NULL CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS')),
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
    PRIMARY KEY (seller_account_id,domain,fact_family),
    CHECK (history_available_to IS NULL OR history_available_from IS NULL OR history_available_to >= history_available_from)
);

CREATE TABLE analysis_readiness_evaluations (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    data_quality_run_id BIGINT NOT NULL REFERENCES data_quality_runs(id) ON DELETE CASCADE,
    candidate_analysis_date DATE NULL,
    evaluated_at TIMESTAMPTZ NOT NULL,
    cutoff_at TIMESTAMPTZ NOT NULL,
    core_gate_status TEXT NOT NULL CHECK (core_gate_status IN ('READY','BLOCKED','INSUFFICIENT_ANALYSIS_HISTORY','CONNECTED_NO_ANALYZABLE_DATA')),
    selected BOOLEAN NOT NULL DEFAULT FALSE,
    blocking_reasons_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    temporal_policy_version TEXT NOT NULL,
    dq_policy_version TEXT NOT NULL,
    capability_policy_version TEXT NOT NULL,
    sales_watermark_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    catalog_watermark_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    evaluation_hash TEXT NOT NULL CHECK (evaluation_hash ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT (data_quality_run_id,candidate_analysis_date)
);

CREATE INDEX idx_dq_runs_seller_cutoff ON data_quality_runs(seller_account_id,cutoff_at DESC,id DESC);
CREATE INDEX idx_dq_issues_run ON data_quality_issues(data_quality_run_id,id);
CREATE UNIQUE INDEX idx_dq_issues_evaluation_identity ON data_quality_issues(data_quality_run_id,domain,dimension,code,period_start,period_end) NULLS NOT DISTINCT;
CREATE INDEX idx_capability_latest_cutoff ON business_capability_assessments(seller_account_id,capability_key,capability_scope,assessed_at DESC,id DESC);
CREATE INDEX idx_readiness_seller_date ON analysis_readiness_evaluations(seller_account_id,candidate_analysis_date DESC,evaluated_at DESC,id DESC);
CREATE INDEX idx_sales_reconciliations_maturity ON sales_reconciliations(seller_account_id,business_date,observed_at DESC,id DESC) WHERE normalized_day_hash IS NOT NULL;
CREATE INDEX idx_sku_versions_as_known ON canonical_sku_versions(canonical_sku_id,observed_at DESC,id DESC);
CREATE INDEX idx_sku_assignments_historical ON sku_category_assignments(canonical_sku_id,observed_from,observed_to,id);
CREATE INDEX idx_sales_account_as_known ON sales_account_daily(seller_account_id,business_date,observed_at DESC,revision_no DESC,id DESC);
CREATE INDEX idx_sales_sku_as_known ON sales_demand_daily(seller_account_id,canonical_sku_id,business_date,observed_at DESC,revision_no DESC,id DESC) WHERE canonical_sku_id IS NOT NULL;
CREATE INDEX idx_posting_versions_as_known ON sales_posting_versions(canonical_posting_id,observed_at DESC,version_no DESC,id DESC);
CREATE INDEX idx_return_events_as_known ON sales_return_events(sales_return_id,observed_at DESC,version_no DESC,id DESC);
CREATE INDEX idx_lifecycle_events_as_known ON sales_lifecycle_events(seller_account_id,observed_at,id);
