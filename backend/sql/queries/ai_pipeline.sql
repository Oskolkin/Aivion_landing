-- name: CreateAIPipelineRun :one
INSERT INTO ai_pipeline_runs (
    seller_account_id,
    metric_date,
    mode,
    trigger,
    status,
    data_pack_schema_version,
    data_pack_hash,
    started_at,
    dispatch_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: GetAIPipelineRunByDispatchID :one
SELECT *
FROM ai_pipeline_runs
WHERE dispatch_id = $1;

-- name: GetAIPipelineRunForUpdate :one
SELECT *
FROM ai_pipeline_runs
WHERE id = $1
FOR UPDATE;

-- name: UpdateAIPipelineRun :one
UPDATE ai_pipeline_runs
SET
    status = COALESCE(sqlc.narg(status)::text, status),
    data_pack_schema_version = COALESCE(sqlc.narg(data_pack_schema_version)::text, data_pack_schema_version),
    data_pack_hash = COALESCE(sqlc.narg(data_pack_hash)::text, data_pack_hash),
    global_context_schema_version = COALESCE(sqlc.narg(global_context_schema_version)::text, global_context_schema_version),
    global_context_hash = COALESCE(sqlc.narg(global_context_hash)::text, global_context_hash),
    global_context_status = COALESCE(sqlc.narg(global_context_status)::text, global_context_status),
    dashboard_run_id = COALESCE(sqlc.narg(dashboard_run_id)::uuid, dashboard_run_id),
    dashboard_data_snapshot_id = COALESCE(sqlc.narg(dashboard_data_snapshot_id)::bigint, dashboard_data_snapshot_id),
    actions_run_id = COALESCE(sqlc.narg(actions_run_id)::bigint, actions_run_id),
    dashboard_status = COALESCE(sqlc.narg(dashboard_status)::text, dashboard_status),
    actions_status = COALESCE(sqlc.narg(actions_status)::text, actions_status),
    error_code = COALESCE(sqlc.narg(error_code)::text, error_code),
    error_message = COALESCE(sqlc.narg(error_message)::text, error_message),
    completed_at = COALESCE(sqlc.narg(completed_at)::timestamptz, completed_at),
    published_at = COALESCE(sqlc.narg(published_at)::timestamptz, published_at),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetLatestPublishedAIPipelineRunBySellerAndDate :one
SELECT *
FROM ai_pipeline_runs
WHERE seller_account_id = $1
  AND metric_date = $2
  AND published_at IS NOT NULL
ORDER BY published_at DESC, id DESC
LIMIT 1;

-- name: GetLatestPublishedAIPipelineRunBySeller :one
SELECT *
FROM ai_pipeline_runs
WHERE seller_account_id = $1
  AND published_at IS NOT NULL
ORDER BY metric_date DESC, published_at DESC, id DESC
LIMIT 1;

-- name: GetPublishedAIPipelineRunBySellerDateAndDataPackHash :one
SELECT *
FROM ai_pipeline_runs
WHERE seller_account_id = $1
  AND metric_date = $2
  AND data_pack_hash = $3
  AND published_at IS NOT NULL
ORDER BY published_at DESC, id DESC
LIMIT 1;

-- name: GetCurrentPublishedSnapshotBySeller :one
SELECT
    p.id AS pipeline_run_id,
    p.seller_account_id,
    p.metric_date,
    p.dashboard_data_snapshot_id,
    p.dashboard_run_id,
    p.actions_run_id,
    p.data_pack_hash,
    p.global_context_hash,
    p.published_at
FROM seller_current_ai_snapshots scs
INNER JOIN ai_pipeline_runs p
    ON p.id = scs.pipeline_run_id
    AND p.seller_account_id = scs.seller_account_id
INNER JOIN ai_dashboard_data_snapshots ds
    ON ds.id = p.dashboard_data_snapshot_id
    AND ds.ai_pipeline_run_id = p.id
    AND ds.seller_account_id = p.seller_account_id
INNER JOIN ai_dashboard_narratives d
    ON d.id = p.dashboard_run_id
    AND d.seller_account_id = p.seller_account_id
    AND d.ai_pipeline_run_id = p.id
INNER JOIN action_generation_runs a
    ON a.id = p.actions_run_id
    AND a.seller_account_id = p.seller_account_id
    AND a.ai_pipeline_run_id = p.id
WHERE scs.seller_account_id = $1
  AND p.published_at IS NOT NULL
  AND p.dashboard_run_id IS NOT NULL
  AND p.actions_run_id IS NOT NULL
  AND p.dashboard_data_snapshot_id IS NOT NULL
  AND p.metric_date = d.metric_date
  AND p.metric_date = a.metric_date
  AND p.metric_date = ds.metric_date
  AND p.data_pack_hash IS NOT NULL
  AND trim(p.data_pack_hash) <> ''
  AND p.global_context_hash IS NOT NULL
  AND trim(p.global_context_hash) <> ''
  AND d.data_pack_hash IS NOT NULL
  AND d.data_pack_hash = p.data_pack_hash
  AND a.data_pack_hash IS NOT NULL
  AND a.data_pack_hash = p.data_pack_hash
  AND ds.data_pack_hash = p.data_pack_hash
  AND d.global_context_hash IS NOT NULL
  AND d.global_context_hash = p.global_context_hash
  AND a.global_context_hash IS NOT NULL
  AND a.global_context_hash = p.global_context_hash
  AND ds.global_context_hash = p.global_context_hash
  AND ds.payload_hash IS NOT NULL
  AND trim(ds.payload_hash) <> ''
  AND ds.status IN ('completed', 'available')
  AND d.status = 'completed'
  AND a.status = 'completed'
  AND COALESCE(a.final_status, 'completed') IN (
    'completed',
    'completed_repaired',
    'completed_partial',
    'completed_after_retry',
    'empty'
  )
LIMIT 1;

-- name: GetPublishedSnapshotBySellerAndPipelineRun :one
SELECT
    p.id AS pipeline_run_id,
    p.seller_account_id,
    p.metric_date,
    p.dashboard_data_snapshot_id,
    p.dashboard_run_id,
    p.actions_run_id,
    p.data_pack_hash,
    p.global_context_hash,
    p.published_at
FROM ai_pipeline_runs p
INNER JOIN ai_dashboard_data_snapshots ds
    ON ds.id = p.dashboard_data_snapshot_id
    AND ds.ai_pipeline_run_id = p.id
    AND ds.seller_account_id = p.seller_account_id
INNER JOIN ai_dashboard_narratives d
    ON d.id = p.dashboard_run_id
    AND d.seller_account_id = p.seller_account_id
    AND d.ai_pipeline_run_id = p.id
INNER JOIN action_generation_runs a
    ON a.id = p.actions_run_id
    AND a.seller_account_id = p.seller_account_id
    AND a.ai_pipeline_run_id = p.id
WHERE p.seller_account_id = $1
  AND p.id = $2
  AND p.published_at IS NOT NULL
  AND p.dashboard_run_id IS NOT NULL
  AND p.actions_run_id IS NOT NULL
  AND p.dashboard_data_snapshot_id IS NOT NULL
  AND p.metric_date = d.metric_date
  AND p.metric_date = a.metric_date
  AND p.metric_date = ds.metric_date
  AND p.data_pack_hash IS NOT NULL
  AND trim(p.data_pack_hash) <> ''
  AND p.global_context_hash IS NOT NULL
  AND trim(p.global_context_hash) <> ''
  AND d.data_pack_hash IS NOT NULL
  AND d.data_pack_hash = p.data_pack_hash
  AND a.data_pack_hash IS NOT NULL
  AND a.data_pack_hash = p.data_pack_hash
  AND ds.data_pack_hash = p.data_pack_hash
  AND d.global_context_hash IS NOT NULL
  AND d.global_context_hash = p.global_context_hash
  AND a.global_context_hash IS NOT NULL
  AND a.global_context_hash = p.global_context_hash
  AND ds.global_context_hash = p.global_context_hash
  AND ds.payload_hash IS NOT NULL
  AND trim(ds.payload_hash) <> ''
  AND ds.status IN ('completed', 'available')
  AND d.status = 'completed'
  AND a.status = 'completed'
  AND COALESCE(a.final_status, 'completed') IN (
    'completed',
    'completed_repaired',
    'completed_partial',
    'completed_after_retry',
    'empty'
  )
LIMIT 1;

-- name: GetSellerCurrentPipelineRunID :one
SELECT pipeline_run_id
FROM seller_current_ai_snapshots
WHERE seller_account_id = $1
LIMIT 1;

-- name: CountAIPipelineRunsForSellerOnDate :one
SELECT COUNT(*)::bigint AS run_count
FROM ai_pipeline_runs
WHERE seller_account_id = $1
  AND metric_date = $2
  AND created_at >= sqlc.arg(day_start)::timestamptz;

-- name: GetRunningAIPipelineRunBySellerAndDate :one
SELECT *
FROM ai_pipeline_runs
WHERE seller_account_id = $1
  AND metric_date = $2
  AND status = 'running'
ORDER BY started_at DESC, id DESC
LIMIT 1;

-- name: MarkStaleAIPipelineRunsFailed :exec
UPDATE ai_pipeline_runs
SET
    status = 'failed',
    error_code = 'stale_running',
    error_message = 'pipeline run exceeded running timeout',
    completed_at = now(),
    updated_at = now()
WHERE status = 'running'
  AND started_at < $1::timestamptz;

-- name: GetAIPipelineRun :one
SELECT *
FROM ai_pipeline_runs
WHERE id = $1;

-- name: ListAIPipelineRunsBySeller :many
SELECT *
FROM ai_pipeline_runs
WHERE seller_account_id = $1
  AND (
    sqlc.narg(metric_date)::date IS NULL
    OR metric_date = sqlc.narg(metric_date)::date
  )
ORDER BY metric_date DESC, created_at DESC
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);

-- name: SetAIPipelinePublishedAt :one
UPDATE ai_pipeline_runs
SET
    published_at = COALESCE(published_at, sqlc.arg(published_at)::timestamptz),
    status = COALESCE(sqlc.narg(status)::text, status),
    completed_at = COALESCE(sqlc.narg(completed_at)::timestamptz, completed_at),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpsertSellerCurrentAISnapshot :exec
INSERT INTO seller_current_ai_snapshots (
    seller_account_id,
    pipeline_run_id,
    updated_at
) VALUES (
    $1, $2, now()
)
ON CONFLICT (seller_account_id)
DO UPDATE SET
    pipeline_run_id = EXCLUDED.pipeline_run_id,
    updated_at = now();

-- name: CreateAIPipelineArtifact :one
INSERT INTO ai_pipeline_artifacts (
    pipeline_run_id,
    kind,
    hash,
    storage,
    object_key,
    payload_json
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ListAIPipelineArtifacts :many
SELECT *
FROM ai_pipeline_artifacts
WHERE pipeline_run_id = $1
ORDER BY created_at ASC, id ASC;

-- name: UpdateAIDashboardNarrativePipelineFields :one
UPDATE ai_dashboard_narratives
SET
    ai_pipeline_run_id = $3,
    data_pack_hash = $4,
    global_context_hash = $5,
    updated_at = now()
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: ClearAIDashboardNarrativePipelineLinkExcept :exec
UPDATE ai_dashboard_narratives
SET
    ai_pipeline_run_id = NULL,
    updated_at = now()
WHERE ai_pipeline_run_id = $1
  AND id <> $2;

-- name: ClearActionGenerationRunPipelineLinkExcept :exec
UPDATE action_generation_runs
SET
    ai_pipeline_run_id = NULL,
    updated_at = now()
WHERE ai_pipeline_run_id = $1
  AND id <> $2;

-- name: UpdateActionGenerationRunPipelineFields :one
UPDATE action_generation_runs
SET
    ai_pipeline_run_id = $2,
    data_pack_hash = $3,
    global_context_hash = $4,
    updated_at = now()
WHERE id = $1
RETURNING *;
