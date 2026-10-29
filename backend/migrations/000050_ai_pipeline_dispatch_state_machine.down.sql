DROP INDEX IF EXISTS idx_ai_pipeline_dispatches_recovery;

ALTER TABLE ai_pipeline_dispatches
    DROP CONSTRAINT IF EXISTS ai_pipeline_dispatches_failed_at_when_failed;
ALTER TABLE ai_pipeline_dispatches
    DROP CONSTRAINT IF EXISTS ai_pipeline_dispatches_completed_at_when_completed;
ALTER TABLE ai_pipeline_dispatches
    DROP CONSTRAINT IF EXISTS ai_pipeline_dispatches_attempts_nonneg;

ALTER TABLE ai_pipeline_dispatches
    DROP COLUMN IF EXISTS last_error_code,
    DROP COLUMN IF EXISTS next_attempt_at,
    DROP COLUMN IF EXISTS last_attempt_at,
    DROP COLUMN IF EXISTS pipeline_started_at,
    DROP COLUMN IF EXISTS enqueue_requested_at;
