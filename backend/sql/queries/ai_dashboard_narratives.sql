-- name: ResetAIDashboardNarrativeForRetry :one
UPDATE ai_dashboard_narratives
SET
    status = 'running',
    started_at = NOW(),
    finished_at = NULL,
    error_message = NULL,
    raw_ai_response = NULL,
    validated_payload = NULL,
    validation_errors = '[]'::jsonb,
    input_tokens = NULL,
    output_tokens = NULL,
    estimated_cost = NULL,
    updated_at = NOW()
WHERE ai_dashboard_narratives.id = $1
  AND ai_dashboard_narratives.seller_account_id = $2
  AND NOT EXISTS (
    SELECT 1
    FROM ai_pipeline_runs p
    WHERE p.dashboard_run_id = ai_dashboard_narratives.id
      AND p.published_at IS NOT NULL
  )
RETURNING *;

-- name: GetAIDashboardNarrativeBySellerAndMetricDate :one
SELECT *
FROM ai_dashboard_narratives
WHERE seller_account_id = $1
  AND metric_date = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: GetAIDashboardNarrativeByPipelineRunID :one
SELECT *
FROM ai_dashboard_narratives
WHERE ai_pipeline_run_id = $1
  AND seller_account_id = $2;

-- name: InsertAIDashboardNarrative :one
INSERT INTO ai_dashboard_narratives (
    seller_account_id,
    metric_date,
    status,
    health_score,
    severity,
    risk_drivers_hash,
    context_hash,
    prompt_version,
    model,
    started_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: CompleteAIDashboardNarrative :one
UPDATE ai_dashboard_narratives
SET
    status = $3,
    raw_ai_response = $4,
    validated_payload = $5,
    validation_errors = $6,
    input_tokens = $7,
    output_tokens = $8,
    estimated_cost = $9,
    error_message = $10,
    finished_at = $11,
    updated_at = NOW()
WHERE ai_dashboard_narratives.id = $1
  AND ai_dashboard_narratives.seller_account_id = $2
  AND NOT EXISTS (
    SELECT 1
    FROM ai_pipeline_runs p
    WHERE p.dashboard_run_id = ai_dashboard_narratives.id
      AND p.published_at IS NOT NULL
  )
RETURNING *;

-- name: FailAIDashboardNarrative :one
UPDATE ai_dashboard_narratives
SET
    status = $3,
    raw_ai_response = $4,
    validation_errors = $5,
    input_tokens = $6,
    output_tokens = $7,
    estimated_cost = $8,
    error_message = $9,
    finished_at = $10,
    updated_at = NOW()
WHERE ai_dashboard_narratives.id = $1
  AND ai_dashboard_narratives.seller_account_id = $2
  AND NOT EXISTS (
    SELECT 1
    FROM ai_pipeline_runs p
    WHERE p.dashboard_run_id = ai_dashboard_narratives.id
      AND p.published_at IS NOT NULL
  )
RETURNING *;

-- name: DeleteAIDashboardNarrativesBySellerMetricDate :exec
DELETE FROM ai_dashboard_narratives
WHERE ai_dashboard_narratives.seller_account_id = $1
  AND ai_dashboard_narratives.metric_date = $2
  AND NOT EXISTS (
    SELECT 1
    FROM ai_pipeline_runs p
    WHERE p.dashboard_run_id = ai_dashboard_narratives.id
      AND p.published_at IS NOT NULL
  );

-- name: GetAIDashboardNarrativeByID :one
SELECT *
FROM ai_dashboard_narratives
WHERE id = $1
  AND seller_account_id = $2;

-- name: GetAIDashboardNarrativeByIDForUpdate :one
SELECT *
FROM ai_dashboard_narratives
WHERE ai_dashboard_narratives.id = $1
  AND ai_dashboard_narratives.seller_account_id = $2
FOR UPDATE;

-- name: UpdateAIDashboardNarrativeValidatedPayload :one
UPDATE ai_dashboard_narratives
SET
    validated_payload = $3,
    validation_errors = COALESCE($4::jsonb, validation_errors),
    updated_at = NOW()
WHERE ai_dashboard_narratives.id = $1
  AND ai_dashboard_narratives.seller_account_id = $2
  AND NOT EXISTS (
    SELECT 1
    FROM ai_pipeline_runs p
    WHERE p.dashboard_run_id = ai_dashboard_narratives.id
      AND p.published_at IS NOT NULL
  )
RETURNING *;
