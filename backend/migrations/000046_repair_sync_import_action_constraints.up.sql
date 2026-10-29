CREATE TABLE IF NOT EXISTS migration_000043_audit (
    id BIGSERIAL PRIMARY KEY,
    category TEXT NOT NULL,
    entity_id BIGINT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

WITH ranked AS (
    SELECT
        id,
        seller_account_id,
        status,
        ROW_NUMBER() OVER (
            PARTITION BY seller_account_id
            ORDER BY
                CASE status WHEN 'running' THEN 0 WHEN 'pending' THEN 1 ELSE 2 END,
                started_at DESC NULLS LAST,
                created_at DESC NULLS LAST,
                id DESC
        ) AS rn
    FROM sync_jobs
    WHERE status IN ('pending', 'running')
),
losers AS (
    SELECT id
    FROM ranked
    WHERE rn > 1
)
UPDATE sync_jobs sj
SET
    status = 'failed',
    error_code = 'migration_duplicate_active_sync',
    error_message = COALESCE(sj.error_message, 'Deactivated by migration 000046 duplicate active sync cleanup'),
    finished_at = COALESCE(sj.finished_at, now()),
    updated_at = now()
FROM losers l
WHERE sj.id = l.id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM sync_jobs
        WHERE status IN ('pending', 'running')
        GROUP BY seller_account_id
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'migration 000046: duplicate active sync_jobs remain';
    END IF;
END $$;

WITH ranked_imports AS (
    SELECT
        id,
        sync_job_id,
        domain,
        status,
        ROW_NUMBER() OVER (
            PARTITION BY sync_job_id, domain
            ORDER BY
                CASE status
                    WHEN 'completed' THEN 0
                    WHEN 'importing' THEN 1
                    WHEN 'fetching' THEN 2
                    WHEN 'retrying' THEN 3
                    WHEN 'pending' THEN 4
                    WHEN 'failed' THEN 5
                    ELSE 6
                END,
                updated_at DESC NULLS LAST,
                created_at DESC NULLS LAST,
                id DESC
        ) AS rn
    FROM import_jobs
),
losers AS (
    SELECT id
    FROM ranked_imports
    WHERE rn > 1
)
UPDATE import_jobs ij
SET
    status = 'failed',
    domain = ij.domain || '__dedup_' || ij.id::text,
    error_message = COALESCE(ij.error_message, 'deactivated by migration 000046 duplicate import cleanup'),
    updated_at = now()
FROM losers l
WHERE ij.id = l.id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM import_jobs
        GROUP BY sync_job_id, domain
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'migration 000046: duplicate import_jobs remain';
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM action_items
        WHERE run_id IS NULL
    ) THEN
        RAISE EXCEPTION 'migration 000046: action_items without run_id remain';
    END IF;
END $$;

WITH ranked_actions AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY run_id, action_key
            ORDER BY created_at DESC NULLS LAST, id DESC
        ) AS rn
    FROM action_items
    WHERE run_id IS NOT NULL
),
losers AS (
    SELECT id
    FROM ranked_actions
    WHERE rn > 1
)
DELETE FROM action_items ai
USING losers l
WHERE ai.id = l.id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM action_items
        GROUP BY run_id, action_key
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'migration 000046: duplicate action_items remain';
    END IF;
END $$;

ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS chk_ai_pipeline_runs_published_has_runs;

ALTER TABLE ai_pipeline_runs
    ADD CONSTRAINT chk_ai_pipeline_runs_published_has_runs CHECK (
        published_at IS NULL
        OR (dashboard_run_id IS NOT NULL AND actions_run_id IS NOT NULL)
    );

ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS fk_ai_pipeline_runs_dashboard_run_id;
ALTER TABLE ai_pipeline_runs
    ADD CONSTRAINT fk_ai_pipeline_runs_dashboard_run_id
    FOREIGN KEY (dashboard_run_id)
    REFERENCES ai_dashboard_narratives(id)
    DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS fk_ai_pipeline_runs_actions_run_id;
ALTER TABLE ai_pipeline_runs
    ADD CONSTRAINT fk_ai_pipeline_runs_actions_run_id
    FOREIGN KEY (actions_run_id)
    REFERENCES action_generation_runs(id)
    DEFERRABLE INITIALLY IMMEDIATE;

CREATE UNIQUE INDEX IF NOT EXISTS idx_sync_jobs_one_active_per_seller
    ON sync_jobs (seller_account_id)
    WHERE status IN ('pending', 'running');

CREATE UNIQUE INDEX IF NOT EXISTS idx_import_jobs_sync_job_domain
    ON import_jobs (sync_job_id, domain);

CREATE UNIQUE INDEX IF NOT EXISTS idx_action_items_run_action_key
    ON action_items (run_id, action_key)
    WHERE run_id IS NOT NULL;
