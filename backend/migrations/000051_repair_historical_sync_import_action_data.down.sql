-- Rollback may be lossy: superseded import jobs and deleted duplicate action_items are not restored.

DROP INDEX IF EXISTS idx_import_jobs_sync_job_domain_active;
CREATE UNIQUE INDEX IF NOT EXISTS idx_import_jobs_sync_job_domain
    ON import_jobs (sync_job_id, domain);

ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS fk_ai_pipeline_runs_dashboard_run_id;
ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS fk_ai_pipeline_runs_actions_run_id;

ALTER TABLE import_jobs DROP CONSTRAINT IF EXISTS import_jobs_status_check;
ALTER TABLE import_jobs
    ADD CONSTRAINT import_jobs_status_check CHECK (
        status IN ('pending', 'fetching', 'importing', 'retrying', 'completed', 'failed')
    );

ALTER TABLE import_jobs
    DROP COLUMN IF EXISTS superseded_by_import_job_id,
    DROP COLUMN IF EXISTS deduplicated_at;

DROP TABLE IF EXISTS migration_repair_audit;
