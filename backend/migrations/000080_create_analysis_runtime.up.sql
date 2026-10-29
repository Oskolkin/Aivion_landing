CREATE TABLE analysis_contract_bundles (
    hash TEXT PRIMARY KEY CHECK (hash ~ '^[0-9a-f]{64}$'),
    schema_version TEXT NOT NULL,
    bundle_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE analysis_runs (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    run_kind TEXT NOT NULL CHECK (run_kind IN ('STANDARD_DAILY','EXPLICIT_REANALYSIS','EVALUATION')),
    execution_class TEXT NOT NULL CHECK (execution_class IN ('PRODUCTION','EVALUATION')),
    status TEXT NOT NULL CHECK (status IN ('PREPARING','RUNNING','READY_TO_PUBLISH','PUBLISHED','NO_OP','FAILED','CANCELLED','STALE')),
    analysis_date DATE NOT NULL,
    analysis_cutoff_at TIMESTAMPTZ NOT NULL,
    factual_universe_id UUID NULL,
    base_publication_id BIGINT NULL,
    contract_bundle_hash TEXT NOT NULL REFERENCES analysis_contract_bundles(hash) ON DELETE RESTRICT,
    run_intent_hash TEXT NOT NULL CHECK (run_intent_hash ~ '^[0-9a-f]{64}$'),
    trigger TEXT NOT NULL,
    readiness_evaluation_id BIGINT NULL REFERENCES analysis_readiness_evaluations(id) ON DELETE RESTRICT,
    current_stage TEXT NULL,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    cancel_requested_at TIMESTAMPTZ NULL,
    cancel_reason_code TEXT NULL,
    error_code TEXT NULL,
    error_message TEXT NULL,
    UNIQUE (id,seller_account_id),
    FOREIGN KEY (factual_universe_id,seller_account_id)
        REFERENCES factual_universes(id,seller_account_id) ON DELETE RESTRICT,
    CHECK (trigger <> ''),
    CHECK (started_at IS NULL OR started_at >= requested_at),
    CHECK (completed_at IS NULL OR completed_at >= requested_at)
);

CREATE UNIQUE INDEX uq_analysis_runs_one_active_production_per_seller
    ON analysis_runs(seller_account_id)
    WHERE execution_class = 'PRODUCTION'
      AND status IN ('PREPARING','RUNNING','READY_TO_PUBLISH');

CREATE INDEX idx_analysis_runs_seller_date
    ON analysis_runs(seller_account_id,analysis_date DESC,requested_at DESC,id DESC);
CREATE INDEX idx_analysis_runs_universe
    ON analysis_runs(factual_universe_id)
    WHERE factual_universe_id IS NOT NULL;

CREATE TABLE analysis_run_stages (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES analysis_runs(id) ON DELETE CASCADE,
    stage_key TEXT NOT NULL,
    stage_version TEXT NOT NULL,
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    status TEXT NOT NULL CHECK (status IN ('PENDING','READY','RUNNING','SUCCEEDED','SUCCEEDED_WITH_LIMITATIONS','FAILED_RETRYABLE','FAILED_TERMINAL','SKIPPED','CANCELLED')),
    input_hash TEXT NULL CHECK (input_hash IS NULL OR input_hash ~ '^[0-9a-f]{64}$'),
    output_hash TEXT NULL CHECK (output_hash IS NULL OR output_hash ~ '^[0-9a-f]{64}$'),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    accepted_attempt_id BIGINT NULL,
    accepted_artifact_id BIGINT NULL,
    lease_owner TEXT NULL,
    lease_token UUID NULL,
    lease_version BIGINT NOT NULL DEFAULT 0 CHECK (lease_version >= 0),
    lease_expires_at TIMESTAMPTZ NULL,
    heartbeat_at TIMESTAMPTZ NULL,
    next_retry_at TIMESTAMPTZ NULL,
    started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    error_code TEXT NULL,
    error_message TEXT NULL,
    UNIQUE (run_id,stage_key),
    UNIQUE (id,run_id),
    CHECK (stage_key <> '')
);

CREATE INDEX idx_analysis_run_stages_runnable
    ON analysis_run_stages(status,next_retry_at,ordinal,id)
    WHERE status IN ('READY','FAILED_RETRYABLE');
CREATE INDEX idx_analysis_run_stages_lease
    ON analysis_run_stages(lease_expires_at)
    WHERE status = 'RUNNING';

CREATE TABLE analysis_stage_attempts (
    id BIGSERIAL PRIMARY KEY,
    stage_id BIGINT NOT NULL REFERENCES analysis_run_stages(id) ON DELETE CASCADE,
    attempt_no INTEGER NOT NULL CHECK (attempt_no > 0),
    worker_id TEXT NOT NULL,
    lease_token UUID NOT NULL,
    lease_version BIGINT NOT NULL CHECK (lease_version >= 0),
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NULL,
    status TEXT NOT NULL,
    failure_class TEXT NULL,
    failure_code TEXT NULL,
    error_summary TEXT NULL,
    input_hash TEXT NOT NULL CHECK (input_hash ~ '^[0-9a-f]{64}$'),
    output_hash TEXT NULL CHECK (output_hash IS NULL OR output_hash ~ '^[0-9a-f]{64}$'),
    output_artifact_id BIGINT NULL,
    UNIQUE (stage_id,attempt_no),
    CHECK (worker_id <> ''),
    CHECK (status <> ''),
    CHECK (finished_at IS NULL OR finished_at >= started_at)
);

CREATE TABLE analysis_ai_attempts (
    id BIGSERIAL PRIMARY KEY,
    stage_attempt_id BIGINT NOT NULL REFERENCES analysis_stage_attempts(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    provider_request_id TEXT NULL,
    input_tokens BIGINT NULL CHECK (input_tokens IS NULL OR input_tokens >= 0),
    output_tokens BIGINT NULL CHECK (output_tokens IS NULL OR output_tokens >= 0),
    reasoning_tokens BIGINT NULL CHECK (reasoning_tokens IS NULL OR reasoning_tokens >= 0),
    cached_tokens BIGINT NULL CHECK (cached_tokens IS NULL OR cached_tokens >= 0),
    total_tokens BIGINT NULL CHECK (total_tokens IS NULL OR total_tokens >= 0),
    latency_ms BIGINT NULL CHECK (latency_ms IS NULL OR latency_ms >= 0),
    finish_reason TEXT NULL,
    incomplete_reason TEXT NULL,
    request_hash TEXT NULL CHECK (request_hash IS NULL OR request_hash ~ '^[0-9a-f]{64}$'),
    response_hash TEXT NULL CHECK (response_hash IS NULL OR response_hash ~ '^[0-9a-f]{64}$'),
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    error_code TEXT NULL,
    error_message TEXT NULL,
    error_metadata_json JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (provider <> ''),
    CHECK (model <> ''),
    CHECK (error_metadata_json IS NULL OR jsonb_typeof(error_metadata_json) = 'object'),
    CHECK (finished_at IS NULL OR started_at IS NULL OR finished_at >= started_at)
);

CREATE TABLE analysis_artifacts (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES analysis_runs(id) ON DELETE CASCADE,
    stage_id BIGINT NULL REFERENCES analysis_run_stages(id) ON DELETE RESTRICT,
    stage_attempt_id BIGINT NULL REFERENCES analysis_stage_attempts(id) ON DELETE RESTRICT,
    stage_key TEXT NOT NULL,
    kind TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    sha256 TEXT NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    media_type TEXT NOT NULL,
    compression TEXT NULL,
    compressed_bytes BIGINT NULL CHECK (compressed_bytes IS NULL OR compressed_bytes >= 0),
    uncompressed_bytes BIGINT NOT NULL CHECK (uncompressed_bytes >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (sha256,storage_key),
    CHECK (stage_key <> ''),
    CHECK (kind <> ''),
    CHECK (storage_key <> '')
);

ALTER TABLE analysis_run_stages
    ADD CONSTRAINT fk_analysis_run_stages_accepted_attempt
        FOREIGN KEY (accepted_attempt_id) REFERENCES analysis_stage_attempts(id)
        ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
    ADD CONSTRAINT fk_analysis_run_stages_accepted_artifact
        FOREIGN KEY (accepted_artifact_id) REFERENCES analysis_artifacts(id)
        ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE analysis_stage_attempts
    ADD CONSTRAINT fk_analysis_stage_attempts_output_artifact
        FOREIGN KEY (output_artifact_id) REFERENCES analysis_artifacts(id)
        ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;

CREATE INDEX idx_analysis_stage_attempts_stage
    ON analysis_stage_attempts(stage_id,attempt_no DESC);
CREATE INDEX idx_analysis_ai_attempts_stage_attempt
    ON analysis_ai_attempts(stage_attempt_id,id);
CREATE INDEX idx_analysis_artifacts_run_stage
    ON analysis_artifacts(run_id,stage_key,id);

CREATE TABLE analysis_run_events (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES analysis_runs(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (event_type <> ''),
    CHECK (jsonb_typeof(payload_json) = 'object')
);

CREATE INDEX idx_analysis_run_events_run
    ON analysis_run_events(run_id,id);

CREATE OR REPLACE FUNCTION forbid_analysis_append_only_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = '55000';
END;
$$;

-- Attempts are append-only for business history: DELETE forbidden; identity
-- columns immutable after insert. Status/finished_at/output fields may update
-- when an attempt completes or is closed by lease reclaim.
CREATE OR REPLACE FUNCTION protect_analysis_stage_attempts()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'analysis_stage_attempts is append-only' USING ERRCODE = '55000';
    END IF;
    IF NEW.stage_id IS DISTINCT FROM OLD.stage_id
        OR NEW.attempt_no IS DISTINCT FROM OLD.attempt_no
        OR NEW.worker_id IS DISTINCT FROM OLD.worker_id
        OR NEW.lease_token IS DISTINCT FROM OLD.lease_token
        OR NEW.lease_version IS DISTINCT FROM OLD.lease_version
        OR NEW.input_hash IS DISTINCT FROM OLD.input_hash
        OR NEW.started_at IS DISTINCT FROM OLD.started_at THEN
        RAISE EXCEPTION 'analysis_stage_attempts identity columns are immutable' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_analysis_contract_bundles_append_only
    BEFORE UPDATE OR DELETE ON analysis_contract_bundles
    FOR EACH ROW EXECUTE FUNCTION forbid_analysis_append_only_mutation();
CREATE TRIGGER trg_analysis_stage_attempts_append_only
    BEFORE UPDATE OR DELETE ON analysis_stage_attempts
    FOR EACH ROW EXECUTE FUNCTION protect_analysis_stage_attempts();
CREATE TRIGGER trg_analysis_ai_attempts_append_only
    BEFORE UPDATE OR DELETE ON analysis_ai_attempts
    FOR EACH ROW EXECUTE FUNCTION forbid_analysis_append_only_mutation();
CREATE TRIGGER trg_analysis_artifacts_append_only
    BEFORE UPDATE OR DELETE ON analysis_artifacts
    FOR EACH ROW EXECUTE FUNCTION forbid_analysis_append_only_mutation();
CREATE TRIGGER trg_analysis_run_events_append_only
    BEFORE UPDATE OR DELETE ON analysis_run_events
    FOR EACH ROW EXECUTE FUNCTION forbid_analysis_append_only_mutation();
