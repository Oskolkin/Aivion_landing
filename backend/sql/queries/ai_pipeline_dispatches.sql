-- name: UpsertAIPipelineDispatch :one
INSERT INTO ai_pipeline_dispatches (
    seller_account_id,
    sync_job_id,
    metric_date,
    stage,
    status,
    task_id,
    attempts,
    last_error,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, 0, $7, now()
)
ON CONFLICT (sync_job_id, stage)
DO UPDATE SET
    seller_account_id = EXCLUDED.seller_account_id,
    metric_date = COALESCE(EXCLUDED.metric_date, ai_pipeline_dispatches.metric_date),
    updated_at = now()
RETURNING *;

-- name: GetAIPipelineDispatchBySyncAndStage :one
SELECT *
FROM ai_pipeline_dispatches
WHERE sync_job_id = $1
  AND stage = $2;

-- name: GetAIPipelineDispatchByID :one
SELECT *
FROM ai_pipeline_dispatches
WHERE id = $1;

-- name: BeginAIPipelineDispatchRecalculation :one
UPDATE ai_pipeline_dispatches
SET
    status = 'recalculation_running',
    updated_at = now()
WHERE id = $1
  AND status IN ('pending', 'retry_pending')
RETURNING *;

-- name: MarkAIPipelineDispatchRecalculationCompleted :one
UPDATE ai_pipeline_dispatches
SET
    status = 'recalculation_completed',
    recalculation_completed_at = now(),
    updated_at = now()
WHERE id = $1
  AND status = 'recalculation_running'
RETURNING *;

-- name: BeginAIPipelineDispatchEnqueue :one
UPDATE ai_pipeline_dispatches
SET
    status = 'enqueue_pending',
    enqueue_requested_at = now(),
    last_attempt_at = now(),
    attempts = attempts + 1,
    last_error = NULL,
    last_error_code = NULL,
    updated_at = now()
WHERE id = $1
  AND status IN ('recalculation_completed', 'retry_pending')
RETURNING *;

-- name: MarkAIPipelineDispatchEnqueued :one
UPDATE ai_pipeline_dispatches
SET
    status = 'enqueued',
    enqueued_at = now(),
    task_id = COALESCE(sqlc.narg(task_id)::text, task_id),
    last_error = NULL,
    last_error_code = NULL,
    updated_at = now()
WHERE id = $1
  AND status IN ('enqueue_pending')
RETURNING *;

-- name: MarkAIPipelineDispatchEnqueueRetry :one
UPDATE ai_pipeline_dispatches
SET
    status = 'retry_pending',
    last_error = $2,
    last_error_code = $3,
    next_attempt_at = $4,
    updated_at = now()
WHERE id = $1
  AND status IN ('enqueue_pending')
RETURNING *;

-- name: BeginAIPipelineDispatchPipelineStarted :one
UPDATE ai_pipeline_dispatches
SET
    status = 'pipeline_started',
    pipeline_started_at = now(),
    pipeline_run_id = COALESCE(sqlc.narg(pipeline_run_id)::bigint, pipeline_run_id),
    updated_at = now()
WHERE sync_job_id = $1
  AND stage = $2
  AND status IN ('enqueued', 'retry_pending')
RETURNING *;

-- name: UpdateAIPipelineDispatchPipelineRunID :one
UPDATE ai_pipeline_dispatches
SET
    pipeline_run_id = $2,
    updated_at = now()
WHERE id = $1
  AND status = 'pipeline_started'
RETURNING *;

-- name: GetAIPipelineDispatchByIDForUpdate :one
SELECT *
FROM ai_pipeline_dispatches
WHERE id = $1
FOR UPDATE;

-- name: ClaimAIPipelineDispatchExecution :one
UPDATE ai_pipeline_dispatches
SET
    status = 'pipeline_started',
    pipeline_started_at = COALESCE(pipeline_started_at, now()),
    pipeline_run_id = $2,
    execution_lease_token = $3,
    lease_acquired_at = $4,
    lease_expires_at = $5,
    pipeline_attempts = pipeline_attempts + 1,
    last_error = NULL,
    last_error_code = NULL,
    updated_at = now()
WHERE id = $1
  AND status IN ('enqueued', 'retry_pending')
  AND (next_attempt_at IS NULL OR next_attempt_at <= $4)
RETURNING *;

-- name: MarkAIPipelineDispatchPipelineRetry :one
UPDATE ai_pipeline_dispatches
SET
    status = 'retry_pending',
    last_error = $2,
    last_error_code = $3,
    next_attempt_at = $4,
    execution_lease_token = NULL,
    lease_acquired_at = NULL,
    lease_expires_at = NULL,
    updated_at = now()
WHERE id = $1
  AND status IN ('pipeline_started', 'enqueued')
  AND (execution_lease_token IS NULL OR execution_lease_token = sqlc.narg(execution_lease_token)::text)
RETURNING *;

-- name: MarkAIPipelineDispatchCompleted :one
UPDATE ai_pipeline_dispatches
SET
    status = 'completed',
    pipeline_run_id = COALESCE(sqlc.narg(pipeline_run_id)::bigint, pipeline_run_id),
    completed_at = now(),
    last_error = NULL,
    last_error_code = NULL,
    execution_lease_token = NULL,
    lease_acquired_at = NULL,
    lease_expires_at = NULL,
    updated_at = now()
WHERE sync_job_id = $1
  AND stage = $2
  AND status IN ('pipeline_started', 'enqueued', 'retry_pending')
RETURNING *;

-- name: MarkAIPipelineDispatchCompletedByID :one
UPDATE ai_pipeline_dispatches
SET
    status = 'completed',
    pipeline_run_id = COALESCE(sqlc.narg(pipeline_run_id)::bigint, pipeline_run_id),
    completed_at = now(),
    last_error = NULL,
    last_error_code = NULL,
    execution_lease_token = NULL,
    lease_acquired_at = NULL,
    lease_expires_at = NULL,
    updated_at = now()
WHERE id = $1
  AND status IN ('pipeline_started', 'enqueued', 'retry_pending')
RETURNING *;

-- name: MarkAIPipelineDispatchFailed :one
UPDATE ai_pipeline_dispatches
SET
    status = 'failed',
    last_error = $2,
    last_error_code = sqlc.narg(last_error_code)::text,
    failed_at = now(),
    updated_at = now()
WHERE id = $1
  AND status NOT IN ('completed', 'failed')
RETURNING *;

-- name: AdminRetryAIPipelineDispatch :one
UPDATE ai_pipeline_dispatches
SET
    status = 'retry_pending',
    failed_at = NULL,
    next_attempt_at = now(),
    last_error = NULL,
    last_error_code = 'admin_retry',
    updated_at = now()
WHERE id = $1
  AND status = 'failed'
RETURNING *;

-- name: ListRecoverableAIPipelineDispatches :many
SELECT *
FROM ai_pipeline_dispatches
WHERE stage = sqlc.arg(stage)
  AND (
    status = 'recalculation_completed'
    OR (
      status = 'enqueue_pending'
      AND enqueue_requested_at IS NOT NULL
      AND enqueue_requested_at <= sqlc.arg(now_ts)::timestamptz - sqlc.arg(enqueue_stuck_interval)::interval
    )
    OR (
      status = 'retry_pending'
      AND (next_attempt_at IS NULL OR next_attempt_at <= sqlc.arg(now_ts)::timestamptz)
    )
    OR (
      status = 'enqueued'
      AND enqueued_at IS NOT NULL
      AND enqueued_at <= sqlc.arg(now_ts)::timestamptz - sqlc.arg(enqueued_timeout)::interval
    )
    OR (
      status = 'pipeline_started'
      AND lease_expires_at IS NOT NULL
      AND lease_expires_at <= sqlc.arg(now_ts)::timestamptz
    )
    OR (
      status = 'pipeline_started'
      AND lease_expires_at IS NULL
      AND (
        pipeline_started_at IS NULL
        OR pipeline_started_at <= sqlc.arg(now_ts)::timestamptz - sqlc.arg(started_timeout)::interval
      )
    )
  )
ORDER BY updated_at ASC, id ASC
LIMIT sqlc.arg(limit_count)
FOR UPDATE SKIP LOCKED;

-- name: ReleaseStaleAIPipelineDispatchExecution :one
UPDATE ai_pipeline_dispatches
SET
    status = 'retry_pending',
    next_attempt_at = sqlc.arg(now_ts)::timestamptz,
    execution_lease_token = NULL,
    lease_acquired_at = NULL,
    lease_expires_at = NULL,
    last_error = COALESCE(sqlc.narg(last_error)::text, last_error),
    last_error_code = COALESCE(sqlc.narg(last_error_code)::text, last_error_code),
    updated_at = now()
WHERE id = $1
  AND status = 'pipeline_started'
  AND (
    (lease_expires_at IS NOT NULL AND lease_expires_at <= sqlc.arg(now_ts)::timestamptz)
    OR (
      lease_expires_at IS NULL
      AND pipeline_started_at IS NOT NULL
      AND pipeline_started_at <= sqlc.arg(now_ts)::timestamptz - sqlc.arg(started_timeout)::interval
    )
  )
RETURNING *;

-- name: ValidateAIPipelineDispatchLease :one
SELECT *
FROM ai_pipeline_dispatches
WHERE id = $1
  AND status = 'pipeline_started'
  AND pipeline_run_id = $2
  AND execution_lease_token = $3
  AND lease_expires_at > $4;

-- name: RenewAIPipelineDispatchLease :one
UPDATE ai_pipeline_dispatches
SET
    lease_expires_at = $2,
    updated_at = now()
WHERE id = $1
  AND status = 'pipeline_started'
  AND execution_lease_token = $3
  AND lease_expires_at > sqlc.arg(now_ts)::timestamptz
RETURNING *;
