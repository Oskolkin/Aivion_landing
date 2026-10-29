CREATE TABLE IF NOT EXISTS seller_current_ai_snapshots (
    seller_account_id BIGINT PRIMARY KEY REFERENCES seller_accounts(id) ON DELETE CASCADE,
    pipeline_run_id BIGINT NOT NULL REFERENCES ai_pipeline_runs(id) ON DELETE RESTRICT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO seller_current_ai_snapshots (seller_account_id, pipeline_run_id, updated_at)
SELECT p.seller_account_id, p.id, now()
FROM ai_pipeline_runs p
INNER JOIN (
    SELECT seller_account_id, MAX(published_at) AS max_published_at
    FROM ai_pipeline_runs
    WHERE published_at IS NOT NULL
    GROUP BY seller_account_id
) latest
    ON latest.seller_account_id = p.seller_account_id
    AND latest.max_published_at = p.published_at
WHERE p.published_at IS NOT NULL
ON CONFLICT (seller_account_id)
DO UPDATE SET pipeline_run_id = EXCLUDED.pipeline_run_id, updated_at = now();

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_pipeline_runs_published_dashboard_run
    ON ai_pipeline_runs (dashboard_run_id)
    WHERE published_at IS NOT NULL AND dashboard_run_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_pipeline_runs_published_actions_run
    ON ai_pipeline_runs (actions_run_id)
    WHERE published_at IS NOT NULL AND actions_run_id IS NOT NULL;
