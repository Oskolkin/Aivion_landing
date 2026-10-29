ALTER TABLE ai_pipeline_dispatches
    ADD COLUMN IF NOT EXISTS pipeline_run_id BIGINT NULL REFERENCES ai_pipeline_runs(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS failed_at TIMESTAMPTZ NULL;
