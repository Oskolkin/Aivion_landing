ALTER TABLE ai_pipeline_dispatches
    DROP COLUMN IF EXISTS failed_at,
    DROP COLUMN IF EXISTS pipeline_run_id;
