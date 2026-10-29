DROP INDEX IF EXISTS idx_ai_pipeline_runs_mode_status;
DROP INDEX IF EXISTS idx_ai_pipeline_runs_provider_container_id;
DROP INDEX IF EXISTS uq_ai_pipeline_runs_provider_response_id;
DROP INDEX IF EXISTS uq_ai_pipeline_runs_one_active_unified_analysis;

ALTER TABLE ai_pipeline_runs
    DROP COLUMN IF EXISTS provider_request_id,
    DROP COLUMN IF EXISTS cleanup_error,
    DROP COLUMN IF EXISTS cleanup_completed_at,
    DROP COLUMN IF EXISTS cleanup_started_at,
    DROP COLUMN IF EXISTS cleanup_status,
    DROP COLUMN IF EXISTS cancel_requested_at,
    DROP COLUMN IF EXISTS provider_attempt_count,
    DROP COLUMN IF EXISTS code_interpreter_calls,
    DROP COLUMN IF EXISTS total_tokens,
    DROP COLUMN IF EXISTS output_tokens,
    DROP COLUMN IF EXISTS input_tokens,
    DROP COLUMN IF EXISTS provider_latency_ms,
    DROP COLUMN IF EXISTS provider_completed_at,
    DROP COLUMN IF EXISTS provider_started_at,
    DROP COLUMN IF EXISTS provider_status,
    DROP COLUMN IF EXISTS provider_response_id,
    DROP COLUMN IF EXISTS provider_container_file_path,
    DROP COLUMN IF EXISTS provider_container_file_id,
    DROP COLUMN IF EXISTS provider_container_id,
    DROP COLUMN IF EXISTS provider_model,
    DROP COLUMN IF EXISTS provider,
    DROP COLUMN IF EXISTS pack_uncompressed_bytes,
    DROP COLUMN IF EXISTS pack_compressed_bytes,
    DROP COLUMN IF EXISTS pack_archive_sha256,
    DROP COLUMN IF EXISTS pack_storage_key,
    DROP COLUMN IF EXISTS pack_hash,
    DROP COLUMN IF EXISTS pack_id,
    DROP COLUMN IF EXISTS prompt_version,
    DROP COLUMN IF EXISTS pipeline_version;

ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS chk_ai_pipeline_runs_status;

ALTER TABLE ai_pipeline_runs
    ADD CONSTRAINT chk_ai_pipeline_runs_status CHECK (
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
    );
