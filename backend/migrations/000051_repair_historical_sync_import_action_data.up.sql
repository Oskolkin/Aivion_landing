-- P0.5 idempotent repair for historical sync/import/action data and safe constraints.
-- Safe after corrected 000043, legacy 000043 (__dedup_ losers), and clean schemas through 000050.

CREATE TABLE IF NOT EXISTS migration_repair_audit (
    id BIGSERIAL PRIMARY KEY,
    migration_version TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    canonical_id BIGINT NULL,
    affected_id BIGINT NULL,
    parent_id BIGINT NULL,
    repair_action TEXT NOT NULL,
    reason_code TEXT NOT NULL,
    old_status TEXT NULL,
    new_status TEXT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_migration_repair_audit_dedupe
    ON migration_repair_audit (migration_version, entity_type, repair_action, affected_id)
    WHERE affected_id IS NOT NULL;

ALTER TABLE import_jobs
    ADD COLUMN IF NOT EXISTS superseded_by_import_job_id BIGINT NULL REFERENCES import_jobs(id),
    ADD COLUMN IF NOT EXISTS deduplicated_at TIMESTAMPTZ NULL;

DROP INDEX IF EXISTS idx_import_jobs_sync_job_domain;
DROP INDEX IF EXISTS idx_import_jobs_sync_job_domain_active;

ALTER TABLE import_jobs DROP CONSTRAINT IF EXISTS import_jobs_status_check;
ALTER TABLE import_jobs
    ADD CONSTRAINT import_jobs_status_check CHECK (
        status IN ('pending', 'fetching', 'importing', 'retrying', 'completed', 'failed', 'superseded')
    );

-- Active sync dedupe (idempotent).
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
),
updated AS (
    UPDATE sync_jobs sj
    SET
        status = 'failed',
        error_code = 'migration_duplicate_active_sync',
        error_message = COALESCE(sj.error_message, 'Deactivated by migration 000051 duplicate active sync cleanup'),
        finished_at = COALESCE(sj.finished_at, now()),
        updated_at = now()
    FROM losers l
    WHERE sj.id = l.id
      AND sj.error_code IS DISTINCT FROM 'migration_duplicate_active_sync'
    RETURNING sj.id, sj.seller_account_id, l.status AS old_status
)
INSERT INTO migration_repair_audit (
    migration_version, entity_type, canonical_id, affected_id, parent_id,
    repair_action, reason_code, old_status, new_status, metadata_json
)
SELECT
    '000051',
    'sync_job',
    NULL,
    u.id,
    u.seller_account_id,
    'mark_failed_duplicate',
    'duplicate_active_sync',
    u.old_status,
    'failed',
    jsonb_build_object('seller_account_id', u.seller_account_id)
FROM updated u
WHERE NOT EXISTS (
    SELECT 1
    FROM migration_repair_audit a
    WHERE a.migration_version = '000051'
      AND a.entity_type = 'sync_job'
      AND a.repair_action = 'mark_failed_duplicate'
      AND a.affected_id = u.id
);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM sync_jobs
        WHERE status IN ('pending', 'running')
        GROUP BY seller_account_id
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'migration 000051: duplicate active sync_jobs remain after cleanup';
    END IF;
END $$;

-- Convert legacy __dedup_ losers from 000043/000046 into superseded rows with restored domain.
WITH legacy_losers AS (
    SELECT
        loser.id AS loser_id,
        loser.sync_job_id,
        loser.status AS old_status,
        regexp_replace(loser.domain, '__dedup_[0-9]+$', '') AS base_domain
    FROM import_jobs loser
    WHERE loser.domain ~ '__dedup_[0-9]+$'
      AND loser.superseded_by_import_job_id IS NULL
      AND loser.status <> 'superseded'
),
paired AS (
    SELECT
        ll.loser_id,
        ll.sync_job_id,
        ll.old_status,
        ll.base_domain,
        canonical.id AS canonical_id
    FROM legacy_losers ll
    INNER JOIN import_jobs canonical
        ON canonical.sync_job_id = ll.sync_job_id
       AND canonical.domain = ll.base_domain
       AND canonical.superseded_by_import_job_id IS NULL
       AND canonical.id <> ll.loser_id
),
updated_legacy AS (
    UPDATE import_jobs ij
    SET
        domain = p.base_domain,
        status = 'superseded',
        superseded_by_import_job_id = p.canonical_id,
        deduplicated_at = COALESCE(ij.deduplicated_at, now()),
        error_message = COALESCE(ij.error_message, 'Superseded by migration 000051 legacy dedup repair'),
        updated_at = now()
    FROM paired p
    WHERE ij.id = p.loser_id
    RETURNING ij.id, p.canonical_id, p.sync_job_id, p.base_domain, p.old_status
)
INSERT INTO migration_repair_audit (
    migration_version, entity_type, canonical_id, affected_id, parent_id,
    repair_action, reason_code, old_status, new_status, metadata_json
)
SELECT
    '000051',
    'import_job',
    u.canonical_id,
    u.id,
    u.sync_job_id,
    'supersede_legacy_dedup',
    'legacy_domain_dedup',
    u.old_status,
    'superseded',
    jsonb_build_object('domain', u.base_domain)
FROM updated_legacy u
WHERE NOT EXISTS (
    SELECT 1
    FROM migration_repair_audit a
    WHERE a.migration_version = '000051'
      AND a.entity_type = 'import_job'
      AND a.repair_action = 'supersede_legacy_dedup'
      AND a.affected_id = u.id
);

-- Import job dedupe for remaining logical duplicates (canonical rows only).
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
                    WHEN 'superseded' THEN 6
                    ELSE 7
                END,
                finished_at DESC NULLS LAST,
                records_imported DESC,
                updated_at DESC NULLS LAST,
                created_at DESC NULLS LAST,
                id DESC
        ) AS rn
    FROM import_jobs
    WHERE superseded_by_import_job_id IS NULL
      AND status <> 'superseded'
      AND domain !~ '__dedup_[0-9]+$'
),
losers AS (
    SELECT id, sync_job_id, domain, status
    FROM ranked_imports
    WHERE rn > 1
),
updated_imports AS (
    UPDATE import_jobs ij
    SET
        status = 'superseded',
        superseded_by_import_job_id = canonical.id,
        deduplicated_at = COALESCE(ij.deduplicated_at, now()),
        error_message = COALESCE(ij.error_message, 'Superseded by migration 000051 duplicate import cleanup'),
        updated_at = now()
    FROM losers l
    INNER JOIN ranked_imports canonical
        ON canonical.sync_job_id = l.sync_job_id
       AND canonical.domain = l.domain
       AND canonical.rn = 1
    WHERE ij.id = l.id
      AND ij.superseded_by_import_job_id IS NULL
    RETURNING ij.id, canonical.id AS canonical_id, l.sync_job_id, l.domain, l.status AS old_status
)
INSERT INTO migration_repair_audit (
    migration_version, entity_type, canonical_id, affected_id, parent_id,
    repair_action, reason_code, old_status, new_status, metadata_json
)
SELECT
    '000051',
    'import_job',
    u.canonical_id,
    u.id,
    u.sync_job_id,
    'supersede_duplicate',
    'duplicate_import_job',
    u.old_status,
    'superseded',
    jsonb_build_object('domain', u.domain)
FROM updated_imports u
WHERE NOT EXISTS (
    SELECT 1
    FROM migration_repair_audit a
    WHERE a.migration_version = '000051'
      AND a.entity_type = 'import_job'
      AND a.repair_action = 'supersede_duplicate'
      AND a.affected_id = u.id
);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM import_jobs
        WHERE superseded_by_import_job_id IS NULL
          AND status <> 'superseded'
          AND domain !~ '__dedup_[0-9]+$'
        GROUP BY sync_job_id, domain
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'migration 000051: duplicate canonical import_jobs remain after cleanup';
    END IF;
END $$;

DROP INDEX IF EXISTS idx_import_jobs_sync_job_domain;
DROP INDEX IF EXISTS idx_import_jobs_sync_job_domain_active;
CREATE UNIQUE INDEX IF NOT EXISTS idx_import_jobs_sync_job_domain_active
    ON import_jobs (sync_job_id, domain)
    WHERE superseded_by_import_job_id IS NULL AND status <> 'superseded';

-- Actions backfill: single unambiguous candidate by seller + metric_date.
UPDATE action_items ai
SET run_id = single.run_id
FROM (
    SELECT ai.id AS item_id, MIN(agr.id) AS run_id
    FROM action_items ai
    INNER JOIN action_generation_runs agr
        ON ai.seller_account_id = agr.seller_account_id
       AND ai.metric_date IS NOT NULL
       AND agr.metric_date = ai.metric_date
    WHERE ai.run_id IS NULL
    GROUP BY ai.id
    HAVING COUNT(DISTINCT agr.id) = 1
) single
WHERE ai.id = single.item_id;

-- Deterministic disambiguation: exactly one pipeline-linked candidate among multiple runs.
UPDATE action_items ai
SET run_id = linked.run_id
FROM (
    SELECT ai.id AS item_id, MIN(agr.id) AS run_id
    FROM action_items ai
    INNER JOIN action_generation_runs agr
        ON ai.seller_account_id = agr.seller_account_id
       AND ai.metric_date IS NOT NULL
       AND agr.metric_date = ai.metric_date
    WHERE ai.run_id IS NULL
      AND agr.ai_pipeline_run_id IS NOT NULL
    GROUP BY ai.id
    HAVING COUNT(DISTINCT agr.id) > 1
       AND COUNT(DISTINCT agr.id) FILTER (WHERE agr.ai_pipeline_run_id IS NOT NULL) = 1
) linked
WHERE ai.id = linked.item_id;

-- Abort on ambiguous action_items without run_id.
DO $$
DECLARE
    ambiguous_count BIGINT;
BEGIN
    SELECT COUNT(*) INTO ambiguous_count
    FROM action_items ai
    WHERE ai.run_id IS NULL
      AND EXISTS (
          SELECT 1
          FROM action_generation_runs agr
          WHERE agr.seller_account_id = ai.seller_account_id
            AND agr.metric_date = ai.metric_date
      );

    IF ambiguous_count > 0 THEN
        RAISE EXCEPTION 'migration 000051: ambiguous action_items run_id backfill (% rows)', ambiguous_count;
    END IF;
END $$;

-- Duplicate (run_id, action_key) cleanup: keep canonical, archive losers in audit.
WITH ranked_actions AS (
    SELECT
        id,
        run_id,
        action_key,
        ROW_NUMBER() OVER (
            PARTITION BY run_id, action_key
            ORDER BY
                updated_at DESC NULLS LAST,
                created_at DESC NULLS LAST,
                id DESC
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
INSERT INTO migration_repair_audit (
    migration_version, entity_type, canonical_id, affected_id, parent_id,
    repair_action, reason_code, metadata_json
)
SELECT
    '000051',
    'action_item',
    NULL,
    d.id,
    d.run_id,
    'delete_duplicate_action_key',
    'duplicate_run_action_key',
    jsonb_build_object('action_key', d.action_key)
FROM deleted d
WHERE NOT EXISTS (
    SELECT 1
    FROM migration_repair_audit a
    WHERE a.migration_version = '000051'
      AND a.entity_type = 'action_item'
      AND a.repair_action = 'delete_duplicate_action_key'
      AND a.affected_id = d.id
);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM action_items
        WHERE run_id IS NOT NULL
        GROUP BY run_id, action_key
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'migration 000051: duplicate action_items remain';
    END IF;
END $$;

-- Orphan pipeline links: unpublished pipelines may drop broken references.
UPDATE ai_pipeline_runs p
SET
    dashboard_run_id = NULL,
    error_code = COALESCE(p.error_code, 'migration_orphan_dashboard_link'),
    error_message = COALESCE(p.error_message, 'Cleared orphan dashboard_run_id by migration 000051'),
    updated_at = now()
WHERE p.published_at IS NULL
  AND p.dashboard_run_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM ai_dashboard_narratives d WHERE d.id = p.dashboard_run_id
  );

UPDATE ai_pipeline_runs p
SET
    actions_run_id = NULL,
    error_code = COALESCE(p.error_code, 'migration_orphan_actions_link'),
    error_message = COALESCE(p.error_message, 'Cleared orphan actions_run_id by migration 000051'),
    updated_at = now()
WHERE p.published_at IS NULL
  AND p.actions_run_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM action_generation_runs a WHERE a.id = p.actions_run_id
  );

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM ai_pipeline_runs p
        WHERE p.published_at IS NOT NULL
          AND (
              (p.dashboard_run_id IS NOT NULL AND NOT EXISTS (
                  SELECT 1 FROM ai_dashboard_narratives d WHERE d.id = p.dashboard_run_id
              ))
              OR
              (p.actions_run_id IS NOT NULL AND NOT EXISTS (
                  SELECT 1 FROM action_generation_runs a WHERE a.id = p.actions_run_id
              ))
          )
    ) THEN
        RAISE EXCEPTION 'migration 000051: published pipeline has orphan dashboard/actions link';
    END IF;
END $$;

-- Re-assert sync/import/action indexes and FK constraints (idempotent).
CREATE UNIQUE INDEX IF NOT EXISTS idx_sync_jobs_one_active_per_seller
    ON sync_jobs (seller_account_id)
    WHERE status IN ('pending', 'running');

CREATE UNIQUE INDEX IF NOT EXISTS idx_action_items_run_action_key
    ON action_items (run_id, action_key)
    WHERE run_id IS NOT NULL;

ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS fk_ai_pipeline_runs_dashboard_run_id;
ALTER TABLE ai_pipeline_runs
    ADD CONSTRAINT fk_ai_pipeline_runs_dashboard_run_id
    FOREIGN KEY (dashboard_run_id)
    REFERENCES ai_dashboard_narratives(id)
    NOT VALID
    DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS fk_ai_pipeline_runs_actions_run_id;
ALTER TABLE ai_pipeline_runs
    ADD CONSTRAINT fk_ai_pipeline_runs_actions_run_id
    FOREIGN KEY (actions_run_id)
    REFERENCES action_generation_runs(id)
    NOT VALID
    DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE ai_pipeline_runs VALIDATE CONSTRAINT fk_ai_pipeline_runs_dashboard_run_id;
ALTER TABLE ai_pipeline_runs VALIDATE CONSTRAINT fk_ai_pipeline_runs_actions_run_id;
