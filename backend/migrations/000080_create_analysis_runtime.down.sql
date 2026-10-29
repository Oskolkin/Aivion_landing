DROP TRIGGER IF EXISTS trg_analysis_run_events_append_only ON analysis_run_events;
DROP TRIGGER IF EXISTS trg_analysis_artifacts_append_only ON analysis_artifacts;
DROP TRIGGER IF EXISTS trg_analysis_ai_attempts_append_only ON analysis_ai_attempts;
DROP TRIGGER IF EXISTS trg_analysis_stage_attempts_append_only ON analysis_stage_attempts;
DROP TRIGGER IF EXISTS trg_analysis_contract_bundles_append_only ON analysis_contract_bundles;

ALTER TABLE IF EXISTS analysis_run_stages
    DROP CONSTRAINT IF EXISTS fk_analysis_run_stages_accepted_attempt,
    DROP CONSTRAINT IF EXISTS fk_analysis_run_stages_accepted_artifact;
ALTER TABLE IF EXISTS analysis_stage_attempts
    DROP CONSTRAINT IF EXISTS fk_analysis_stage_attempts_output_artifact;

DROP TABLE IF EXISTS analysis_run_events;
DROP TABLE IF EXISTS analysis_ai_attempts;
DROP TABLE IF EXISTS analysis_artifacts;
DROP TABLE IF EXISTS analysis_stage_attempts;
DROP TABLE IF EXISTS analysis_run_stages;
DROP TABLE IF EXISTS analysis_runs;
DROP TABLE IF EXISTS analysis_contract_bundles;
DROP FUNCTION IF EXISTS protect_analysis_stage_attempts();
DROP FUNCTION IF EXISTS forbid_analysis_append_only_mutation();
