-- One dashboard narrative row per pipeline run (versioned rows per pipeline).
CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_dashboard_narratives_pipeline_run
    ON ai_dashboard_narratives (ai_pipeline_run_id)
    WHERE ai_pipeline_run_id IS NOT NULL;

-- One actions generation run per pipeline run.
CREATE UNIQUE INDEX IF NOT EXISTS uq_action_generation_runs_pipeline_run
    ON action_generation_runs (ai_pipeline_run_id)
    WHERE ai_pipeline_run_id IS NOT NULL;
