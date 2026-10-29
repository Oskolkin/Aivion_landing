-- name: CreateUnifiedAnalysisRun :one
INSERT INTO ai_pipeline_runs (
    seller_account_id,
    metric_date,
    mode,
    trigger,
    status,
    data_pack_schema_version,
    data_pack_hash,
    pipeline_version,
    prompt_version,
    provider,
    provider_model,
    provider_attempt_count,
    started_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING *;

-- name: GetUnifiedAnalysisRunByID :one
SELECT *
FROM ai_pipeline_runs
WHERE id = $1
  AND mode = 'unified_analysis';

-- name: GetUnifiedAnalysisRunByIDForUpdate :one
SELECT *
FROM ai_pipeline_runs
WHERE id = $1
  AND mode = 'unified_analysis'
FOR UPDATE;

-- name: FindActiveUnifiedAnalysisRun :one
SELECT *
FROM ai_pipeline_runs
WHERE seller_account_id = $1
  AND metric_date = $2
  AND mode = 'unified_analysis'
  AND status NOT IN (
      'failed',
      'cancelled',
      'provider_completed',
      'publication_ready',
      'publishing',
      'published',
      'publication_blocked',
      'publication_failed',
      'cleanup_completed'
  )
ORDER BY id DESC
LIMIT 1;

-- name: TransitionUnifiedAnalysisRunStatus :one
UPDATE ai_pipeline_runs
SET
    status = sqlc.arg(next_status),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND mode = 'unified_analysis'
  AND status = sqlc.arg(expected_status)
RETURNING *;

-- name: SaveUnifiedAnalysisPack :one
UPDATE ai_pipeline_runs
SET
    pack_id = $2,
    pack_hash = $3,
    pack_storage_key = $4,
    pack_archive_sha256 = $5,
    pack_compressed_bytes = $6,
    pack_uncompressed_bytes = $7,
    data_pack_hash = $3,
    data_pack_schema_version = $8,
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
RETURNING *;

-- name: SaveUnifiedAnalysisContainer :one
UPDATE ai_pipeline_runs
SET
    provider_container_id = $2,
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
RETURNING *;

-- name: SaveUnifiedAnalysisContainerFile :one
UPDATE ai_pipeline_runs
SET
    provider_container_file_id = $2,
    provider_container_file_path = $3,
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
RETURNING *;

-- name: SaveUnifiedAnalysisResponseStarted :one
UPDATE ai_pipeline_runs
SET
    provider_response_id = $2,
    provider_status = $3,
    provider_started_at = $4,
    provider_request_id = COALESCE(sqlc.narg(provider_request_id)::text, provider_request_id),
    provider_attempt_count = provider_attempt_count + 1,
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
RETURNING *;

-- name: SaveUnifiedAnalysisResponseCompleted :one
UPDATE ai_pipeline_runs
SET
    status = $2,
    provider_status = $3,
    provider_completed_at = $4,
    provider_latency_ms = $5,
    input_tokens = $6,
    output_tokens = $7,
    total_tokens = $8,
    code_interpreter_calls = $9,
    cleanup_status = COALESCE(sqlc.narg(cleanup_status)::text, cleanup_status),
    completed_at = COALESCE(sqlc.narg(completed_at)::timestamptz, completed_at),
    error_code = COALESCE(sqlc.narg(error_code)::text, error_code),
    error_message = COALESCE(sqlc.narg(error_message)::text, error_message),
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
RETURNING *;

-- name: RequestUnifiedAnalysisCancellation :one
UPDATE ai_pipeline_runs
SET
    cancel_requested_at = COALESCE(cancel_requested_at, now()),
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
RETURNING *;

-- name: MarkUnifiedAnalysisFailed :one
UPDATE ai_pipeline_runs
SET
    status = 'failed',
    error_code = $2,
    error_message = $3,
    provider_status = COALESCE(sqlc.narg(provider_status)::text, provider_status),
    completed_at = COALESCE(completed_at, now()),
    cleanup_status = COALESCE(sqlc.narg(cleanup_status)::text, cleanup_status),
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
RETURNING *;

-- name: SaveUnifiedAnalysisProviderTerminalFailure :one
-- Persists terminal provider fields for incomplete/failed provider responses
-- without overloading MarkFailed semantics.
UPDATE ai_pipeline_runs
SET
    status = 'failed',
    provider_status = $2,
    provider_completed_at = $3,
    provider_latency_ms = $4,
    input_tokens = $5,
    output_tokens = $6,
    total_tokens = $7,
    code_interpreter_calls = $8,
    error_code = $9,
    error_message = $10,
    cleanup_status = COALESCE(sqlc.narg(cleanup_status)::text, cleanup_status),
    completed_at = COALESCE(completed_at, now()),
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
RETURNING *;

-- name: StartNextUnifiedAnalysisProviderAttempt :one
-- CAS: restart provider response only while still provider_running on the
-- expected response id and under the attempt budget. Increments attempt_count.
UPDATE ai_pipeline_runs
SET
    provider_response_id = $3,
    provider_status = $4,
    provider_request_id = $5,
    provider_started_at = $6,
    provider_attempt_count = provider_attempt_count + 1,
    provider_completed_at = NULL,
    provider_latency_ms = NULL,
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
  AND status = 'provider_running'
  AND provider_response_id = $2
  AND provider_attempt_count < $7
RETURNING *;

-- name: SaveUnifiedAnalysisPublicationBlocked :one
UPDATE ai_pipeline_runs
SET
    status = 'publication_blocked',
    error_code = $2,
    error_message = $3,
    completed_at = COALESCE(completed_at, now()),
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
  AND status = 'output_validating'
RETURNING *;

-- name: MarkUnifiedAnalysisCancelled :one
UPDATE ai_pipeline_runs
SET
    status = 'cancelled',
    provider_status = COALESCE(sqlc.narg(provider_status)::text, provider_status),
    completed_at = COALESCE(completed_at, now()),
    cleanup_status = COALESCE(sqlc.narg(cleanup_status)::text, cleanup_status),
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
RETURNING *;

-- name: SaveUnifiedAnalysisCleanupResult :one
UPDATE ai_pipeline_runs
SET
    cleanup_status = $2,
    cleanup_started_at = COALESCE(sqlc.narg(cleanup_started_at)::timestamptz, cleanup_started_at),
    cleanup_completed_at = COALESCE(sqlc.narg(cleanup_completed_at)::timestamptz, cleanup_completed_at),
    cleanup_error = sqlc.narg(cleanup_error),
    status = COALESCE(sqlc.narg(status)::text, status),
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
RETURNING *;

-- name: UpdateUnifiedAnalysisProviderStatus :one
UPDATE ai_pipeline_runs
SET
    provider_status = $2,
    status = COALESCE(sqlc.narg(status)::text, status),
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
RETURNING *;

-- name: ListRecoverableUnifiedAnalysisRuns :many
-- publication_ready and publishing are always recoverable (Stage 5 publish must be
-- (re-)enqueued regardless of cleanup_status); publication_blocked is only recoverable
-- while cleanup is still outstanding.
SELECT *
FROM ai_pipeline_runs
WHERE mode = 'unified_analysis'
  AND (
    status IN (
      'pack_building',
      'pack_validating',
      'pack_ready',
      'provider_container_creating',
      'provider_container_ready',
      'provider_file_uploading',
      'provider_file_ready',
      'provider_response_starting',
      'provider_running',
      'cleanup_pending',
      'cleanup_running',
      'output_validating'
    )
    OR status = 'provider_completed'
    OR status IN ('publication_ready', 'publishing')
    OR (
      status = 'publication_blocked'
      AND (cleanup_status IS NULL OR cleanup_status IN ('pending', 'running', 'failed'))
    )
  )
ORDER BY id ASC
LIMIT $1;
