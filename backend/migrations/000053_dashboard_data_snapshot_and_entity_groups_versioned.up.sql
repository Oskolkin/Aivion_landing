-- P0.1/P0.2: immutable dashboard data snapshots and versioned action_entity_groups.

CREATE TABLE IF NOT EXISTS ai_dashboard_data_snapshots (
    id BIGSERIAL PRIMARY KEY,
    ai_pipeline_run_id BIGINT NOT NULL REFERENCES ai_pipeline_runs(id) ON DELETE RESTRICT,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    metric_date DATE NOT NULL,
    data_pack_hash TEXT NOT NULL,
    global_context_hash TEXT NOT NULL,
    schema_version TEXT NOT NULL DEFAULT 'dashboard_data_snapshot_v1',
    payload_json JSONB NOT NULL,
    payload_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'completed',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ NULL,
    CONSTRAINT ai_dashboard_data_snapshots_status_check
        CHECK (status IN ('pending', 'running', 'completed', 'failed', 'available')),
    CONSTRAINT ai_dashboard_data_snapshots_completed_hashes_check
        CHECK (
            status NOT IN ('completed', 'available')
            OR (
                data_pack_hash IS NOT NULL
                AND global_context_hash IS NOT NULL
                AND payload_hash IS NOT NULL
                AND length(trim(data_pack_hash)) > 0
                AND length(trim(global_context_hash)) > 0
                AND length(trim(payload_hash)) > 0
            )
        )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_dashboard_data_snapshots_pipeline_run_id
    ON ai_dashboard_data_snapshots (ai_pipeline_run_id);

ALTER TABLE ai_pipeline_runs
    ADD COLUMN IF NOT EXISTS dashboard_data_snapshot_id BIGINT NULL
        REFERENCES ai_dashboard_data_snapshots(id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_pipeline_runs_dashboard_data_snapshot_id
    ON ai_pipeline_runs (dashboard_data_snapshot_id)
    WHERE dashboard_data_snapshot_id IS NOT NULL;

ALTER TABLE action_entity_groups
    ADD COLUMN IF NOT EXISTS action_generation_run_id BIGINT NULL
        REFERENCES action_generation_runs(id) ON DELETE CASCADE;

-- Backfill entity groups from action items (unambiguous run per group_key).
UPDATE action_entity_groups g
SET action_generation_run_id = mapped.run_id
FROM (
    SELECT ai.seller_account_id, ai.entity_group_ref AS group_key, MIN(ai.run_id) AS run_id
    FROM action_items ai
    WHERE ai.run_id IS NOT NULL
      AND ai.entity_group_ref IS NOT NULL
      AND trim(ai.entity_group_ref) <> ''
    GROUP BY ai.seller_account_id, ai.entity_group_ref
    HAVING COUNT(DISTINCT ai.run_id) = 1
) mapped
WHERE g.seller_account_id = mapped.seller_account_id
  AND g.group_key = mapped.group_key
  AND g.action_generation_run_id IS NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT seller_account_id, group_key
        FROM action_entity_groups
        WHERE action_generation_run_id IS NULL
        GROUP BY seller_account_id, group_key
    ) THEN
        RAISE EXCEPTION 'migration 053: ambiguous action_entity_groups backfill';
    END IF;
END $$;

ALTER TABLE action_entity_groups
    ALTER COLUMN action_generation_run_id SET NOT NULL;

ALTER TABLE action_entity_groups
    DROP CONSTRAINT IF EXISTS action_entity_groups_seller_account_id_group_key_key;
DROP INDEX IF EXISTS action_entity_groups_seller_account_id_group_key_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_action_entity_groups_run_group_key
    ON action_entity_groups (action_generation_run_id, group_key);

-- Backfill dashboard data snapshots for already-published pipelines from pipeline-linked narrative hash metadata.
INSERT INTO ai_dashboard_data_snapshots (
    ai_pipeline_run_id,
    seller_account_id,
    metric_date,
    data_pack_hash,
    global_context_hash,
    schema_version,
    payload_json,
    payload_hash,
    status,
    finished_at
)
SELECT
    p.id,
    p.seller_account_id,
    p.metric_date,
    p.data_pack_hash,
    COALESCE(p.global_context_hash, ''),
    'dashboard_data_snapshot_v1',
    jsonb_build_object(
        'schema_version', 'dashboard_data_snapshot_v1',
        'metric_date', to_char(p.metric_date, 'YYYY-MM-DD'),
        'backfill_source', 'published_pipeline_v053',
        'dashboard_metrics', jsonb_build_object(
            'seller_account_id', p.seller_account_id,
            'as_of_date', to_char(p.metric_date, 'YYYY-MM-DD'),
            'account', jsonb_build_object('revenue_today', 0, 'orders_today', 0),
            'skus', '[]'::jsonb
        )
    ),
    'sha256:backfill:' || p.id::text,
    'completed',
    COALESCE(p.published_at, now())
FROM ai_pipeline_runs p
WHERE p.published_at IS NOT NULL
  AND p.dashboard_run_id IS NOT NULL
  AND p.data_pack_hash IS NOT NULL
  AND trim(p.data_pack_hash) <> ''
  AND NOT EXISTS (
      SELECT 1 FROM ai_dashboard_data_snapshots s WHERE s.ai_pipeline_run_id = p.id
  );

UPDATE ai_pipeline_runs p
SET dashboard_data_snapshot_id = s.id
FROM ai_dashboard_data_snapshots s
WHERE s.ai_pipeline_run_id = p.id
  AND p.dashboard_data_snapshot_id IS NULL;

ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS chk_ai_pipeline_runs_published_has_runs;
ALTER TABLE ai_pipeline_runs
    ADD CONSTRAINT chk_ai_pipeline_runs_published_has_runs
        CHECK (
            published_at IS NULL
            OR (
                dashboard_run_id IS NOT NULL
                AND actions_run_id IS NOT NULL
                AND dashboard_data_snapshot_id IS NOT NULL
                AND data_pack_hash IS NOT NULL
                AND trim(data_pack_hash) <> ''
            )
        );

-- Immutability: dashboard data snapshots linked to published pipelines.
CREATE OR REPLACE FUNCTION guard_ai_dashboard_data_snapshot_immutable()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF EXISTS (
            SELECT 1 FROM ai_pipeline_runs p
            WHERE p.dashboard_data_snapshot_id = OLD.id
              AND p.published_at IS NOT NULL
        ) THEN
            RAISE EXCEPTION 'published dashboard data snapshot is immutable';
        END IF;
        RETURN OLD;
    END IF;
    IF TG_OP = 'UPDATE' THEN
        IF EXISTS (
            SELECT 1 FROM ai_pipeline_runs p
            WHERE p.dashboard_data_snapshot_id = OLD.id
              AND p.published_at IS NOT NULL
        ) THEN
            RAISE EXCEPTION 'published dashboard data snapshot is immutable';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_ai_dashboard_data_snapshots_immutable ON ai_dashboard_data_snapshots;
CREATE TRIGGER trg_ai_dashboard_data_snapshots_immutable
    BEFORE UPDATE OR DELETE ON ai_dashboard_data_snapshots
    FOR EACH ROW
    EXECUTE FUNCTION guard_ai_dashboard_data_snapshot_immutable();

-- Immutability: entity groups for published actions runs.
CREATE OR REPLACE FUNCTION guard_action_entity_groups_published_immutable()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    run_published BOOLEAN;
BEGIN
    IF TG_OP = 'DELETE' THEN
        SELECT EXISTS (
            SELECT 1
            FROM action_generation_runs r
            INNER JOIN ai_pipeline_runs p ON p.id = r.ai_pipeline_run_id
            WHERE r.id = OLD.action_generation_run_id
              AND p.published_at IS NOT NULL
        ) INTO run_published;
        IF run_published THEN
            RAISE EXCEPTION 'published entity group is immutable';
        END IF;
        RETURN OLD;
    END IF;

    IF TG_OP = 'UPDATE' THEN
        SELECT EXISTS (
            SELECT 1
            FROM action_generation_runs r
            INNER JOIN ai_pipeline_runs p ON p.id = r.ai_pipeline_run_id
            WHERE r.id = OLD.action_generation_run_id
              AND p.published_at IS NOT NULL
        ) INTO run_published;
        IF run_published THEN
            RAISE EXCEPTION 'published entity group is immutable';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_action_entity_groups_published_immutable ON action_entity_groups;
CREATE TRIGGER trg_action_entity_groups_published_immutable
    BEFORE UPDATE OR DELETE ON action_entity_groups
    FOR EACH ROW
    EXECUTE FUNCTION guard_action_entity_groups_published_immutable();
