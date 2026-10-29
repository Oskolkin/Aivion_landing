-- Normalize legacy periodic_sync rows before tightening type constraint.
ALTER TABLE sync_jobs
    ADD COLUMN IF NOT EXISTS error_code TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE import_jobs
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE sync_jobs
SET type = 'incremental_sync'
WHERE type = 'periodic_sync';

ALTER TABLE sync_jobs DROP CONSTRAINT IF EXISTS sync_jobs_type_check;
ALTER TABLE sync_jobs
    ADD CONSTRAINT sync_jobs_type_check CHECK (type IN ('initial_sync', 'incremental_sync'));

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
    SELECT id, seller_account_id, status
    FROM ranked
    WHERE rn > 1
)
UPDATE sync_jobs sj
SET
    status = 'failed',
    error_code = 'migration_duplicate_active_sync',
    error_message = 'Deactivated by migration 000043 duplicate active sync cleanup',
    finished_at = now(),
    updated_at = now()
FROM losers l
WHERE sj.id = l.id;

INSERT INTO migration_000043_audit (category, entity_id, details)
SELECT
    'sync_jobs_dedup_loser',
    l.id,
    jsonb_build_object('seller_account_id', l.seller_account_id, 'status', l.status)
FROM (
    SELECT id, seller_account_id, status
    FROM sync_jobs
    WHERE error_code = 'migration_duplicate_active_sync'
) l;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM sync_jobs
        WHERE status IN ('pending', 'running')
        GROUP BY seller_account_id
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'migration 000043: duplicate active sync_jobs remain after cleanup';
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_sync_jobs_one_active_per_seller
    ON sync_jobs (seller_account_id)
    WHERE status IN ('pending', 'running');

ALTER TABLE import_jobs DROP CONSTRAINT IF EXISTS import_jobs_status_check;
ALTER TABLE import_jobs
    ADD CONSTRAINT import_jobs_status_check CHECK (
        status IN ('pending', 'fetching', 'importing', 'retrying', 'completed', 'failed')
    );

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
    SELECT id, sync_job_id, domain, status
    FROM ranked_imports
    WHERE rn > 1
)
UPDATE import_jobs ij
SET
    status = 'failed',
    error_message = COALESCE(
        ij.error_message,
        'deactivated by migration 000043 duplicate import cleanup'
    ),
    domain = ij.domain || '__dedup_' || ij.id::text,
    updated_at = now()
FROM losers l
WHERE ij.id = l.id;

INSERT INTO migration_000043_audit (category, entity_id, details)
SELECT
    'import_jobs_dedup_loser',
    l.id,
    jsonb_build_object('sync_job_id', l.sync_job_id, 'domain', l.domain, 'status', l.status)
FROM (
    SELECT id, sync_job_id, domain, status
    FROM import_jobs
    WHERE domain LIKE '%__dedup_%'
) l;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM import_jobs
        GROUP BY sync_job_id, domain
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'migration 000043: duplicate import_jobs remain after cleanup';
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_import_jobs_sync_job_domain
    ON import_jobs (sync_job_id, domain);

ALTER TABLE ai_pipeline_runs
    ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_pipeline_runs_one_running_per_seller_date
    ON ai_pipeline_runs (seller_account_id, metric_date)
    WHERE status = 'running';

-- action_items: version snapshots per generation run.
ALTER TABLE action_items DROP CONSTRAINT IF EXISTS action_items_seller_account_id_action_key_key;

-- Backfill run_id for legacy rows using seller+metric_date.
UPDATE action_items ai
SET run_id = agr.id
FROM action_generation_runs agr
WHERE ai.run_id IS NULL
  AND ai.seller_account_id = agr.seller_account_id
  AND ai.metric_date IS NOT NULL
  AND agr.metric_date = ai.metric_date;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM action_items
        WHERE run_id IS NULL
    ) THEN
        RAISE EXCEPTION 'migration 000043: action_items without run_id remain (ambiguous backfill)';
    END IF;
END $$;

WITH ranked_actions AS (
    SELECT
        id,
        run_id,
        action_key,
        ROW_NUMBER() OVER (
            PARTITION BY run_id, action_key
            ORDER BY created_at DESC NULLS LAST, id DESC
        ) AS rn
    FROM action_items
    WHERE run_id IS NOT NULL
),
losers AS (
    SELECT id, run_id, action_key
    FROM ranked_actions
    WHERE rn > 1
),
deleted AS (
    DELETE FROM action_items ai
    USING losers l
    WHERE ai.id = l.id
    RETURNING ai.id, ai.run_id, ai.action_key
)
INSERT INTO migration_000043_audit (category, entity_id, details)
SELECT
    'action_items_dedup_loser',
    d.id,
    jsonb_build_object('run_id', d.run_id, 'action_key', d.action_key)
FROM deleted d;

CREATE UNIQUE INDEX IF NOT EXISTS idx_action_items_run_action_key
    ON action_items (run_id, action_key)
    WHERE run_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS ai_usage_events (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    pipeline_run_id BIGINT NULL REFERENCES ai_pipeline_runs(id) ON DELETE SET NULL,
    component TEXT NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 0,
    model TEXT NULL,
    request_id TEXT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    estimated_cost NUMERIC(18, 6) NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'completed',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_usage_events_pipeline
    ON ai_usage_events (pipeline_run_id, component, attempt);

CREATE INDEX IF NOT EXISTS idx_ai_usage_events_seller_created
    ON ai_usage_events (seller_account_id, created_at DESC);
