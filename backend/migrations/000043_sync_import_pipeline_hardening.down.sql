DROP TABLE IF EXISTS ai_usage_events;

DROP INDEX IF EXISTS idx_action_items_run_action_key;

-- Rollback to seller+action_key uniqueness: keep only the latest snapshot per pair.
-- Older versioned rows are removed intentionally during down migration.
DELETE FROM action_items ai
WHERE ai.id IN (
    SELECT id
    FROM (
        SELECT
            id,
            ROW_NUMBER() OVER (
                PARTITION BY seller_account_id, action_key
                ORDER BY created_at DESC NULLS LAST, id DESC
            ) AS rn
        FROM action_items
    ) ranked
    WHERE ranked.rn > 1
);

ALTER TABLE action_items
    ADD CONSTRAINT action_items_seller_account_id_action_key_key UNIQUE (seller_account_id, action_key);

DROP INDEX IF EXISTS idx_ai_pipeline_runs_one_running_per_seller_date;

ALTER TABLE ai_pipeline_runs DROP COLUMN IF EXISTS published_at;

DROP INDEX IF EXISTS idx_import_jobs_sync_job_domain;

ALTER TABLE import_jobs DROP CONSTRAINT IF EXISTS import_jobs_status_check;
ALTER TABLE import_jobs
    ADD CONSTRAINT import_jobs_status_check CHECK (
        status IN ('pending', 'fetching', 'importing', 'completed', 'failed')
    );

DROP INDEX IF EXISTS idx_sync_jobs_one_active_per_seller;

ALTER TABLE sync_jobs DROP CONSTRAINT IF EXISTS sync_jobs_type_check;
ALTER TABLE sync_jobs
    ADD CONSTRAINT sync_jobs_type_check CHECK (type IN ('initial_sync', 'incremental_sync'));
