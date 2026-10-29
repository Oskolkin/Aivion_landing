-- Unified Analysis Provider (Stage 3): extend ai_pipeline_runs for Code Interpreter runs.
-- Reuses ai_pipeline_runs; does not create a separate run table.

ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS chk_ai_pipeline_runs_status;

ALTER TABLE ai_pipeline_runs
    ADD CONSTRAINT chk_ai_pipeline_runs_status CHECK (
        status IN (
            -- legacy seller AI pipeline
            'running',
            'completed',
            'completed_partial',
            'failed',
            'context_failed',
            'dashboard_failed',
            'actions_failed',
            'empty',
            -- unified analysis provider (stage 3)
            'created',
            'pack_building',
            'pack_validating',
            'pack_ready',
            'provider_container_creating',
            'provider_container_ready',
            'provider_file_uploading',
            'provider_file_ready',
            'provider_response_starting',
            'provider_running',
            'provider_completed',
            'cleanup_pending',
            'cleanup_running',
            'cleanup_completed',
            'cancelled',
            -- reserved for later stages (unused in stage 3)
            'output_validating',
            'publication_ready',
            'publishing',
            'published',
            'publication_blocked'
        )
    );

ALTER TABLE ai_pipeline_runs
    ADD COLUMN IF NOT EXISTS pipeline_version TEXT,
    ADD COLUMN IF NOT EXISTS prompt_version TEXT,
    ADD COLUMN IF NOT EXISTS pack_id TEXT,
    ADD COLUMN IF NOT EXISTS pack_hash TEXT,
    ADD COLUMN IF NOT EXISTS pack_storage_key TEXT,
    ADD COLUMN IF NOT EXISTS pack_archive_sha256 TEXT,
    ADD COLUMN IF NOT EXISTS pack_compressed_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS pack_uncompressed_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS provider TEXT,
    ADD COLUMN IF NOT EXISTS provider_model TEXT,
    ADD COLUMN IF NOT EXISTS provider_container_id TEXT,
    ADD COLUMN IF NOT EXISTS provider_container_file_id TEXT,
    ADD COLUMN IF NOT EXISTS provider_container_file_path TEXT,
    ADD COLUMN IF NOT EXISTS provider_response_id TEXT,
    ADD COLUMN IF NOT EXISTS provider_status TEXT,
    ADD COLUMN IF NOT EXISTS provider_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS provider_completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS provider_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS input_tokens BIGINT,
    ADD COLUMN IF NOT EXISTS output_tokens BIGINT,
    ADD COLUMN IF NOT EXISTS total_tokens BIGINT,
    ADD COLUMN IF NOT EXISTS code_interpreter_calls INTEGER,
    ADD COLUMN IF NOT EXISTS provider_attempt_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cancel_requested_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cleanup_status TEXT,
    ADD COLUMN IF NOT EXISTS cleanup_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cleanup_completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cleanup_error TEXT,
    ADD COLUMN IF NOT EXISTS provider_request_id TEXT;

-- One active unified_analysis run per seller+metric_date.
CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_pipeline_runs_one_active_unified_analysis
    ON ai_pipeline_runs (seller_account_id, metric_date)
    WHERE mode = 'unified_analysis'
      AND status NOT IN ('failed', 'cancelled', 'provider_completed', 'cleanup_completed');

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_pipeline_runs_provider_response_id
    ON ai_pipeline_runs (provider_response_id)
    WHERE provider_response_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ai_pipeline_runs_provider_container_id
    ON ai_pipeline_runs (provider_container_id)
    WHERE provider_container_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ai_pipeline_runs_mode_status
    ON ai_pipeline_runs (mode, status);
