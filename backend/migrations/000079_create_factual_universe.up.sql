CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE factual_universes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    analysis_date DATE NOT NULL,
    analysis_cutoff_at TIMESTAMPTZ NOT NULL,
    manifest_hash TEXT NOT NULL CHECK (manifest_hash ~ '^[0-9a-f]{64}$'),
    manifest_schema_version TEXT NOT NULL,
    manifest_artifact_id BIGINT NULL,
    temporal_policy_version TEXT NOT NULL,
    capability_policy_version TEXT NOT NULL,
    dq_policy_version TEXT NOT NULL,
    factual_api_version TEXT NOT NULL,
    universe_policy_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_account_id,analysis_date,analysis_cutoff_at,manifest_hash),
    UNIQUE (id,seller_account_id)
);

CREATE TABLE factual_universe_sources (
    universe_id UUID NOT NULL,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    fact_family TEXT NOT NULL,
    source_binding_key TEXT NOT NULL,
    source_state TEXT NOT NULL CHECK (source_state IN ('SUCCESS','EMPTY_VALID','PARTIAL','FAILED','NOT_ATTEMPTED')),
    last_accepted_domain_run_id BIGINT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
    coverage_from DATE NULL,
    coverage_to DATE NULL,
    snapshot_as_of TIMESTAMPTZ NULL,
    latest_observed_at TIMESTAMPTZ NULL,
    latest_complete_business_date DATE NULL,
    latest_mature_business_date DATE NULL,
    latest_complete_snapshot_at TIMESTAMPTZ NULL,
    source_contract_key TEXT NOT NULL,
    source_contract_version TEXT NOT NULL,
    source_contract_hash TEXT NOT NULL CHECK (source_contract_hash ~ '^[0-9a-f]{64}$'),
    semantic_mapping_hash TEXT NULL CHECK (semantic_mapping_hash IS NULL OR semantic_mapping_hash ~ '^[0-9a-f]{64}$'),
    source_hash TEXT NOT NULL CHECK (source_hash ~ '^[0-9a-f]{64}$'),
    binding_hash TEXT NOT NULL CHECK (binding_hash ~ '^[0-9a-f]{64}$'),
    PRIMARY KEY (universe_id,domain,fact_family,source_binding_key),
    FOREIGN KEY (universe_id,seller_account_id)
        REFERENCES factual_universes(id,seller_account_id) ON DELETE CASCADE,
    CHECK (domain <> ''),
    CHECK (fact_family <> ''),
    CHECK (source_binding_key <> ''),
    CHECK (coverage_to IS NULL OR coverage_from IS NULL OR coverage_to >= coverage_from)
);

CREATE TABLE factual_universe_capabilities (
    universe_id UUID NOT NULL REFERENCES factual_universes(id) ON DELETE CASCADE,
    capability_key TEXT NOT NULL,
    scope TEXT NOT NULL CHECK (scope IN ('CURRENT_COLLECTION','HISTORICAL_QUERY')),
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
    policy_version TEXT NOT NULL,
    assessment_id BIGINT NULL REFERENCES business_capability_assessments(id) ON DELETE RESTRICT,
    assessment_hash TEXT NOT NULL CHECK (assessment_hash ~ '^[0-9a-f]{64}$'),
    assessed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (universe_id,capability_key,scope)
);

CREATE TABLE factual_universe_dq_summary (
    universe_id UUID NOT NULL REFERENCES factual_universes(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    fact_family TEXT NOT NULL,
    dq_run_id BIGINT NULL REFERENCES data_quality_runs(id) ON DELETE RESTRICT,
    dq_policy_version TEXT NOT NULL,
    dq_result_hash TEXT NOT NULL CHECK (dq_result_hash ~ '^[0-9a-f]{64}$'),
    highest_severity TEXT NOT NULL CHECK (highest_severity IN ('NONE','INFO','WARNING','ERROR','CRITICAL')),
    blocking_issue_count BIGINT NOT NULL DEFAULT 0 CHECK (blocking_issue_count >= 0),
    limitation_issue_count BIGINT NOT NULL DEFAULT 0 CHECK (limitation_issue_count >= 0),
    warning_count BIGINT NOT NULL DEFAULT 0 CHECK (warning_count >= 0),
    canonical_issue_set_hash TEXT NOT NULL CHECK (canonical_issue_set_hash ~ '^[0-9a-f]{64}$'),
    summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (universe_id,domain,fact_family),
    CHECK (domain <> ''),
    CHECK (fact_family <> ''),
    CHECK (jsonb_typeof(summary_json) = 'object')
);

CREATE TABLE factual_universe_entity_sets (
    universe_id UUID NOT NULL REFERENCES factual_universes(id) ON DELETE CASCADE,
    entity_kind TEXT NOT NULL,
    set_hash TEXT NOT NULL CHECK (set_hash ~ '^[0-9a-f]{64}$'),
    member_count BIGINT NOT NULL CHECK (member_count >= 0),
    reference_json JSONB NULL,
    PRIMARY KEY (universe_id,entity_kind),
    CHECK (entity_kind <> '')
);

CREATE INDEX idx_factual_universes_seller_date
    ON factual_universes(seller_account_id,analysis_date DESC,analysis_cutoff_at DESC,created_at DESC);
CREATE INDEX idx_factual_universe_sources_domain_run
    ON factual_universe_sources(last_accepted_domain_run_id)
    WHERE last_accepted_domain_run_id IS NOT NULL;
CREATE INDEX idx_factual_universe_capabilities_assessment
    ON factual_universe_capabilities(assessment_id)
    WHERE assessment_id IS NOT NULL;
CREATE INDEX idx_factual_universe_dq_run
    ON factual_universe_dq_summary(dq_run_id)
    WHERE dq_run_id IS NOT NULL;

CREATE OR REPLACE FUNCTION forbid_factual_universe_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'factual universe is immutable' USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER trg_factual_universes_immutable
    BEFORE UPDATE ON factual_universes
    FOR EACH ROW
    EXECUTE FUNCTION forbid_factual_universe_update();

-- DELETE of a universe referenced by any analysis_run is forbidden (FK RESTRICT
-- on analysis_runs.factual_universe_id also applies once that table exists).
CREATE OR REPLACE FUNCTION forbid_factual_universe_delete_when_referenced()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF to_regclass('public.analysis_runs') IS NOT NULL THEN
        IF EXISTS (
            SELECT 1 FROM analysis_runs r WHERE r.factual_universe_id = OLD.id
        ) THEN
            RAISE EXCEPTION 'factual universe is referenced by analysis_runs' USING ERRCODE = '23503';
        END IF;
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER trg_factual_universes_forbid_referenced_delete
    BEFORE DELETE ON factual_universes
    FOR EACH ROW
    EXECUTE FUNCTION forbid_factual_universe_delete_when_referenced();
