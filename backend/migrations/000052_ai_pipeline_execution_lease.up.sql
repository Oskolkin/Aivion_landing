-- P0.3: execution lease, dispatch↔pipeline 1:1 correlation, separate pipeline attempt counter.

ALTER TABLE ai_pipeline_dispatches
    ADD COLUMN IF NOT EXISTS execution_lease_token TEXT NULL,
    ADD COLUMN IF NOT EXISTS lease_acquired_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS pipeline_attempts INTEGER NOT NULL DEFAULT 0;

ALTER TABLE ai_pipeline_dispatches
    DROP CONSTRAINT IF EXISTS ai_pipeline_dispatches_pipeline_attempts_nonneg;
ALTER TABLE ai_pipeline_dispatches
    ADD CONSTRAINT ai_pipeline_dispatches_pipeline_attempts_nonneg CHECK (pipeline_attempts >= 0);

ALTER TABLE ai_pipeline_runs
    ADD COLUMN IF NOT EXISTS dispatch_id BIGINT NULL REFERENCES ai_pipeline_dispatches(id) ON DELETE SET NULL;

-- Backfill dispatch_id from dispatch.pipeline_run_id where unambiguous.
UPDATE ai_pipeline_runs r
SET dispatch_id = d.id
FROM ai_pipeline_dispatches d
WHERE d.pipeline_run_id = r.id
  AND r.dispatch_id IS NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT dispatch_id
        FROM ai_pipeline_runs
        WHERE dispatch_id IS NOT NULL
        GROUP BY dispatch_id
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'migration 000052: duplicate ai_pipeline_runs per dispatch_id';
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_pipeline_runs_dispatch_id
    ON ai_pipeline_runs (dispatch_id)
    WHERE dispatch_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_pipeline_dispatches_pipeline_run_id
    ON ai_pipeline_dispatches (pipeline_run_id)
    WHERE pipeline_run_id IS NOT NULL;

DROP INDEX IF EXISTS idx_ai_pipeline_dispatches_lease_recovery;
CREATE INDEX idx_ai_pipeline_dispatches_lease_recovery
    ON ai_pipeline_dispatches (lease_expires_at, updated_at, id)
    WHERE status = 'pipeline_started' AND lease_expires_at IS NOT NULL;
