CREATE TABLE ai_pipeline_runs (
    id BIGSERIAL PRIMARY KEY,

    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    metric_date DATE NOT NULL,

    mode TEXT NOT NULL DEFAULT 'unified',
    trigger TEXT NOT NULL DEFAULT 'manual',

    status TEXT NOT NULL,

    data_pack_schema_version TEXT,
    data_pack_hash TEXT NOT NULL,

    global_context_schema_version TEXT,
    global_context_hash TEXT,
    global_context_status TEXT,

    dashboard_run_id UUID NULL,
    actions_run_id BIGINT NULL,

    dashboard_status TEXT,
    actions_status TEXT,

    error_code TEXT,
    error_message TEXT,

    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_ai_pipeline_runs_status CHECK (
        status IN (
            'running',
            'completed',
            'completed_partial',
            'failed',
            'context_failed',
            'dashboard_failed',
            'actions_failed',
            'empty'
        )
    )
);

CREATE INDEX idx_ai_pipeline_runs_seller_date
    ON ai_pipeline_runs (seller_account_id, metric_date DESC, created_at DESC);

CREATE INDEX idx_ai_pipeline_runs_status
    ON ai_pipeline_runs (status);

CREATE INDEX idx_ai_pipeline_runs_data_pack_hash
    ON ai_pipeline_runs (data_pack_hash);

CREATE INDEX idx_ai_pipeline_runs_global_context_hash
    ON ai_pipeline_runs (global_context_hash);

CREATE TABLE ai_pipeline_artifacts (
    id BIGSERIAL PRIMARY KEY,
    pipeline_run_id BIGINT NOT NULL REFERENCES ai_pipeline_runs(id) ON DELETE CASCADE,

    kind TEXT NOT NULL,
    hash TEXT,
    storage TEXT NOT NULL DEFAULT 'db',
    object_key TEXT,

    payload_json JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ai_pipeline_artifacts_run
    ON ai_pipeline_artifacts (pipeline_run_id);

CREATE INDEX idx_ai_pipeline_artifacts_kind
    ON ai_pipeline_artifacts (kind);

ALTER TABLE ai_dashboard_narratives
    ADD COLUMN ai_pipeline_run_id BIGINT NULL REFERENCES ai_pipeline_runs(id) ON DELETE SET NULL,
    ADD COLUMN data_pack_hash TEXT NULL,
    ADD COLUMN global_context_hash TEXT NULL;

CREATE INDEX idx_ai_dashboard_narratives_pipeline_run
    ON ai_dashboard_narratives (ai_pipeline_run_id);

ALTER TABLE action_generation_runs
    ADD COLUMN ai_pipeline_run_id BIGINT NULL REFERENCES ai_pipeline_runs(id) ON DELETE SET NULL,
    ADD COLUMN data_pack_hash TEXT NULL,
    ADD COLUMN global_context_hash TEXT NULL;

CREATE INDEX idx_action_generation_runs_pipeline_run
    ON action_generation_runs (ai_pipeline_run_id);
