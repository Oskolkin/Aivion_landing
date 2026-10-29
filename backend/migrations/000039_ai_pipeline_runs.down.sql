DROP INDEX IF EXISTS idx_action_generation_runs_pipeline_run;
ALTER TABLE action_generation_runs
    DROP COLUMN IF EXISTS global_context_hash,
    DROP COLUMN IF EXISTS data_pack_hash,
    DROP COLUMN IF EXISTS ai_pipeline_run_id;

DROP INDEX IF EXISTS idx_ai_dashboard_narratives_pipeline_run;
ALTER TABLE ai_dashboard_narratives
    DROP COLUMN IF EXISTS global_context_hash,
    DROP COLUMN IF EXISTS data_pack_hash,
    DROP COLUMN IF EXISTS ai_pipeline_run_id;

DROP INDEX IF EXISTS idx_ai_pipeline_artifacts_kind;
DROP INDEX IF EXISTS idx_ai_pipeline_artifacts_run;
DROP TABLE IF EXISTS ai_pipeline_artifacts;

DROP INDEX IF EXISTS idx_ai_pipeline_runs_global_context_hash;
DROP INDEX IF EXISTS idx_ai_pipeline_runs_data_pack_hash;
DROP INDEX IF EXISTS idx_ai_pipeline_runs_status;
DROP INDEX IF EXISTS idx_ai_pipeline_runs_seller_date;
DROP TABLE IF EXISTS ai_pipeline_runs;
