DROP INDEX IF EXISTS idx_ai_pipeline_dispatches_lease_recovery;
DROP INDEX IF EXISTS uq_ai_pipeline_dispatches_pipeline_run_id;
DROP INDEX IF EXISTS uq_ai_pipeline_runs_dispatch_id;

ALTER TABLE ai_pipeline_runs DROP COLUMN IF EXISTS dispatch_id;

ALTER TABLE ai_pipeline_dispatches
    DROP CONSTRAINT IF EXISTS ai_pipeline_dispatches_pipeline_attempts_nonneg;

ALTER TABLE ai_pipeline_dispatches
    DROP COLUMN IF EXISTS execution_lease_token,
    DROP COLUMN IF EXISTS lease_acquired_at,
    DROP COLUMN IF EXISTS lease_expires_at,
    DROP COLUMN IF EXISTS pipeline_attempts;
