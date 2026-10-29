DELETE FROM sync_jobs WHERE type = 'incremental_sync';

DELETE FROM admin_action_logs WHERE action_type = 'seed_created';

ALTER TABLE admin_action_logs
    DROP CONSTRAINT IF EXISTS chk_admin_action_logs_action_type;

ALTER TABLE admin_action_logs
    ADD CONSTRAINT chk_admin_action_logs_action_type CHECK (
        action_type IN (
            'rerun_sync',
            'reset_cursor',
            'rerun_metrics',
            'rerun_alerts',
            'rerun_recommendations',
            'update_billing_state',
            'view_raw_ai_payload'
        )
    );

ALTER TABLE sync_jobs
    DROP CONSTRAINT IF EXISTS sync_jobs_type_check;

ALTER TABLE sync_jobs
    ADD CONSTRAINT sync_jobs_type_check CHECK (type IN ('initial_sync'));
