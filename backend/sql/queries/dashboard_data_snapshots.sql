-- name: GetAIDashboardDataSnapshotByPipelineRunID :one
SELECT *
FROM ai_dashboard_data_snapshots
WHERE ai_pipeline_run_id = $1
LIMIT 1;

-- name: GetAIDashboardDataSnapshotByID :one
SELECT *
FROM ai_dashboard_data_snapshots
WHERE id = $1
LIMIT 1;

-- name: InsertAIDashboardDataSnapshot :one
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
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (ai_pipeline_run_id) DO UPDATE SET
    seller_account_id = EXCLUDED.seller_account_id,
    metric_date = EXCLUDED.metric_date,
    data_pack_hash = EXCLUDED.data_pack_hash,
    global_context_hash = EXCLUDED.global_context_hash,
    schema_version = EXCLUDED.schema_version,
    payload_json = EXCLUDED.payload_json,
    payload_hash = EXCLUDED.payload_hash,
    status = EXCLUDED.status,
    finished_at = EXCLUDED.finished_at
WHERE NOT EXISTS (
    SELECT 1
    FROM ai_pipeline_runs p
    WHERE p.dashboard_data_snapshot_id = ai_dashboard_data_snapshots.id
      AND p.published_at IS NOT NULL
)
RETURNING *;

-- name: LinkAIPipelineRunDashboardDataSnapshot :one
UPDATE ai_pipeline_runs
SET dashboard_data_snapshot_id = $2
WHERE id = $1
  AND published_at IS NULL
  AND (
      dashboard_data_snapshot_id IS NULL
      OR dashboard_data_snapshot_id = $2
  )
RETURNING *;
