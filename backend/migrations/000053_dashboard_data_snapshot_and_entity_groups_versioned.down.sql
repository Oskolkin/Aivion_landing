DROP TRIGGER IF EXISTS trg_action_entity_groups_published_immutable ON action_entity_groups;
DROP FUNCTION IF EXISTS guard_action_entity_groups_published_immutable();
DROP TRIGGER IF EXISTS trg_ai_dashboard_data_snapshots_immutable ON ai_dashboard_data_snapshots;
DROP FUNCTION IF EXISTS guard_ai_dashboard_data_snapshot_immutable();

ALTER TABLE ai_pipeline_runs DROP CONSTRAINT IF EXISTS chk_ai_pipeline_runs_published_has_runs;
ALTER TABLE ai_pipeline_runs
    ADD CONSTRAINT chk_ai_pipeline_runs_published_has_runs
        CHECK (
            published_at IS NULL
            OR (dashboard_run_id IS NOT NULL AND actions_run_id IS NOT NULL)
        );

ALTER TABLE ai_pipeline_runs DROP COLUMN IF EXISTS dashboard_data_snapshot_id;

DROP INDEX IF EXISTS uq_action_entity_groups_run_group_key;
CREATE UNIQUE INDEX IF NOT EXISTS action_entity_groups_seller_account_id_group_key_key
    ON action_entity_groups (seller_account_id, group_key);

ALTER TABLE action_entity_groups DROP COLUMN IF EXISTS action_generation_run_id;

DROP TABLE IF EXISTS ai_dashboard_data_snapshots;
