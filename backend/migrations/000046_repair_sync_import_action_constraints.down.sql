ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS chk_ai_pipeline_runs_published_has_runs;

ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS fk_ai_pipeline_runs_dashboard_run_id;

ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS fk_ai_pipeline_runs_actions_run_id;
