-- Prompt 16: eval harness schema + chat provider telemetry (non user-visible).

ALTER TABLE chat_turns
    ADD COLUMN IF NOT EXISTS telemetry_json JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE eval_suites (
    id BIGSERIAL PRIMARY KEY,
    suite_key TEXT NOT NULL,
    version TEXT NOT NULL,
    dataset_kind TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    rubric_version TEXT NOT NULL,
    manifest_artifact_key TEXT,
    manifest_artifact_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_eval_suites_key_version UNIQUE (suite_key, version),
    CONSTRAINT chk_eval_suites_dataset_kind CHECK (dataset_kind IN (
        'FACTUAL_GOLDEN',
        'CAPABILITY_MATRIX',
        'AI_GOLDEN',
        'LONGITUDINAL',
        'CHAT_GOLDEN',
        'ATTACHMENT_GOLDEN',
        'REAL_SELLER_REPLAY',
        'GRADER_CALIBRATION'
    ))
);

CREATE TABLE eval_cases (
    id BIGSERIAL PRIMARY KEY,
    case_key TEXT NOT NULL,
    version TEXT NOT NULL,
    fixture_artifact_key TEXT,
    fixture_artifact_hash TEXT,
    analysis_date DATE,
    analysis_cutoff_at TIMESTAMPTZ,
    expected_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    tags_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_eval_cases_key_version UNIQUE (case_key, version)
);

CREATE TABLE eval_suite_cases (
    suite_id BIGINT NOT NULL REFERENCES eval_suites(id) ON DELETE CASCADE,
    case_id BIGINT NOT NULL REFERENCES eval_cases(id) ON DELETE CASCADE,
    split TEXT NOT NULL,
    ordinal INTEGER NOT NULL DEFAULT 0,
    critical BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (suite_id, case_id),
    CONSTRAINT chk_eval_suite_cases_split CHECK (split IN ('DEV', 'HOLDOUT', 'CALIBRATION', 'REPLAY'))
);

CREATE INDEX idx_eval_suite_cases_split ON eval_suite_cases (suite_id, split, ordinal);

CREATE TABLE eval_runs (
    id BIGSERIAL PRIMARY KEY,
    suite_id BIGINT NOT NULL REFERENCES eval_suites(id) ON DELETE RESTRICT,
    split TEXT NOT NULL,
    purpose TEXT NOT NULL DEFAULT 'ACCEPTANCE',
    status TEXT NOT NULL,
    verdict TEXT NOT NULL DEFAULT 'PENDING',
    code_sha TEXT NOT NULL DEFAULT '',
    ucdm_version TEXT NOT NULL DEFAULT '',
    factual_api_version TEXT NOT NULL DEFAULT '',
    temporal_policy_version TEXT NOT NULL DEFAULT '',
    capability_policy_version TEXT NOT NULL DEFAULT '',
    materiality_policy_version TEXT NOT NULL DEFAULT '',
    discovery_prompt_version TEXT NOT NULL DEFAULT '',
    discovery_schema_version TEXT NOT NULL DEFAULT '',
    discovery_model_policy_version TEXT NOT NULL DEFAULT '',
    evidence_catalog_version TEXT NOT NULL DEFAULT '',
    deep_prompt_version TEXT NOT NULL DEFAULT '',
    deep_schema_version TEXT NOT NULL DEFAULT '',
    deep_model_policy_version TEXT NOT NULL DEFAULT '',
    action_catalog_version TEXT NOT NULL DEFAULT '',
    action_continuity_policy_version TEXT NOT NULL DEFAULT '',
    projection_version TEXT NOT NULL DEFAULT '',
    chat_prompt_version TEXT NOT NULL DEFAULT '',
    chat_tool_policy_version TEXT NOT NULL DEFAULT '',
    chat_model_policy_version TEXT NOT NULL DEFAULT '',
    contract_bundle_hash TEXT NOT NULL DEFAULT '',
    resolved_models_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    reasoning_config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    grader_versions_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    critical_failures_count INTEGER NOT NULL DEFAULT 0,
    aggregate_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    report_artifact_key TEXT,
    report_artifact_hash TEXT,
    CONSTRAINT chk_eval_runs_split CHECK (split IN ('DEV', 'HOLDOUT', 'CALIBRATION', 'REPLAY')),
    CONSTRAINT chk_eval_runs_status CHECK (status IN (
        'PENDING', 'RUNNING', 'PASSED', 'FAILED', 'HUMAN_REVIEW_PENDING', 'BUDGET_EXCEEDED', 'DATASET_PENDING'
    )),
    CONSTRAINT chk_eval_runs_verdict CHECK (verdict IN (
        'PENDING', 'PASS', 'PASS_WITH_KNOWN_LIMITATIONS', 'FAIL'
    ))
);

CREATE INDEX idx_eval_runs_suite_started ON eval_runs (suite_id, started_at DESC);

CREATE TABLE eval_case_runs (
    id BIGSERIAL PRIMARY KEY,
    eval_run_id BIGINT NOT NULL REFERENCES eval_runs(id) ON DELETE CASCADE,
    case_id BIGINT NOT NULL REFERENCES eval_cases(id) ON DELETE RESTRICT,
    split TEXT NOT NULL,
    status TEXT NOT NULL,
    verdict TEXT NOT NULL DEFAULT 'PENDING',
    analysis_run_id BIGINT REFERENCES analysis_runs(id) ON DELETE SET NULL,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cached_tokens BIGINT NOT NULL DEFAULT 0,
    reasoning_tokens BIGINT NOT NULL DEFAULT 0,
    estimated_cost_usd NUMERIC,
    pricing_version TEXT,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    artifact_key TEXT,
    artifact_hash TEXT,
    error_code TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT chk_eval_case_runs_split CHECK (split IN ('DEV', 'HOLDOUT', 'CALIBRATION', 'REPLAY')),
    CONSTRAINT chk_eval_case_runs_status CHECK (status IN (
        'PENDING', 'RUNNING', 'PASSED', 'FAILED', 'SKIPPED', 'BUDGET_EXCEEDED', 'DATASET_PENDING'
    ))
);

CREATE INDEX idx_eval_case_runs_run ON eval_case_runs (eval_run_id, id);

CREATE TABLE eval_grader_results (
    id BIGSERIAL PRIMARY KEY,
    case_run_id BIGINT NOT NULL REFERENCES eval_case_runs(id) ON DELETE CASCADE,
    grader_key TEXT NOT NULL,
    grader_version TEXT NOT NULL,
    grader_prompt_hash TEXT,
    grader_model TEXT,
    rubric_version TEXT,
    score_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    critical_fail BOOLEAN NOT NULL DEFAULT FALSE,
    artifact_key TEXT,
    artifact_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_eval_grader_results_case ON eval_grader_results (case_run_id, grader_key);

CREATE TABLE eval_human_reviews (
    id BIGSERIAL PRIMARY KEY,
    case_run_id BIGINT NOT NULL REFERENCES eval_case_runs(id) ON DELETE CASCADE,
    reviewer TEXT NOT NULL,
    status TEXT NOT NULL,
    ratings_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    notes TEXT NOT NULL DEFAULT '',
    packet_artifact_key TEXT,
    packet_artifact_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_eval_human_reviews_status CHECK (status IN ('PENDING', 'PASS', 'FAIL', 'NEEDS_DISCUSSION'))
);

CREATE INDEX idx_eval_human_reviews_case ON eval_human_reviews (case_run_id, created_at DESC);

CREATE TABLE eval_baselines (
    id BIGSERIAL PRIMARY KEY,
    suite_id BIGINT NOT NULL REFERENCES eval_suites(id) ON DELETE RESTRICT,
    split TEXT NOT NULL,
    run_id BIGINT NOT NULL REFERENCES eval_runs(id) ON DELETE RESTRICT,
    promoted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    promoted_by TEXT NOT NULL,
    CONSTRAINT uq_eval_baselines_current UNIQUE (suite_id, split),
    CONSTRAINT chk_eval_baselines_split CHECK (split IN ('DEV', 'HOLDOUT', 'CALIBRATION', 'REPLAY'))
);
