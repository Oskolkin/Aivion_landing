ALTER TABLE ai_pipeline_dispatches
    ADD COLUMN IF NOT EXISTS enqueue_requested_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS pipeline_started_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS last_error_code TEXT NULL;

-- Normalize legacy rows created before guarded transitions.
UPDATE ai_pipeline_dispatches
SET status = 'recalculation_completed'
WHERE status = 'pending'
  AND recalculation_completed_at IS NOT NULL;

UPDATE ai_pipeline_dispatches
SET status = 'retry_pending',
    next_attempt_at = COALESCE(next_attempt_at, updated_at)
WHERE status = 'pending'
  AND recalculation_completed_at IS NULL
  AND last_error IS NOT NULL;

ALTER TABLE ai_pipeline_dispatches
    DROP CONSTRAINT IF EXISTS ai_pipeline_dispatches_attempts_nonneg;
ALTER TABLE ai_pipeline_dispatches
    ADD CONSTRAINT ai_pipeline_dispatches_attempts_nonneg CHECK (attempts >= 0);

ALTER TABLE ai_pipeline_dispatches
    DROP CONSTRAINT IF EXISTS ai_pipeline_dispatches_completed_at_when_completed;
ALTER TABLE ai_pipeline_dispatches
    ADD CONSTRAINT ai_pipeline_dispatches_completed_at_when_completed
    CHECK (status <> 'completed' OR completed_at IS NOT NULL);

ALTER TABLE ai_pipeline_dispatches
    DROP CONSTRAINT IF EXISTS ai_pipeline_dispatches_failed_at_when_failed;
ALTER TABLE ai_pipeline_dispatches
    ADD CONSTRAINT ai_pipeline_dispatches_failed_at_when_failed
    CHECK (status <> 'failed' OR failed_at IS NOT NULL);

DROP INDEX IF EXISTS idx_ai_pipeline_dispatches_recovery;
CREATE INDEX idx_ai_pipeline_dispatches_recovery
    ON ai_pipeline_dispatches (next_attempt_at, updated_at, id)
    WHERE status IN (
        'recalculation_completed',
        'enqueue_pending',
        'retry_pending',
        'enqueued',
        'pipeline_started'
    );
