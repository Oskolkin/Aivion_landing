-- name: CreateAIDailySummaryRun :one
INSERT INTO ai_daily_summary_runs (
    seller_account_id,
    metric_date,
    status,
    category_total_count,
    category_completed_count,
    category_failed_count,
    prompt_version,
    model,
    data_quality_notes
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: UpdateAIDailySummaryRun :exec
UPDATE ai_daily_summary_runs
SET
    status = $3,
    account_summary_id = COALESCE($4, account_summary_id),
    category_total_count = $5,
    category_completed_count = $6,
    category_failed_count = $7,
    estimated_input_tokens = COALESCE($8, estimated_input_tokens),
    estimated_output_tokens = COALESCE($9, estimated_output_tokens),
    estimated_cost = COALESCE($10, estimated_cost),
    error_message = $11,
    data_quality_notes = $12,
    updated_at = NOW(),
    completed_at = $13
WHERE id = $1
  AND seller_account_id = $2;

-- name: GetAIDailySummaryRunBySellerAndMetricDate :one
SELECT *
FROM ai_daily_summary_runs
WHERE seller_account_id = $1
  AND metric_date = $2;

-- name: InsertAIDailySummary :one
INSERT INTO ai_daily_summaries (
    run_id,
    seller_account_id,
    metric_date,
    summary_type,
    scope_type,
    scope_id,
    scope_name,
    status,
    previous_summary_id,
    prompt_version,
    model
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: GetAIDailySummaryByScope :one
SELECT *
FROM ai_daily_summaries
WHERE seller_account_id = $1
  AND metric_date = $2
  AND summary_type = $3
  AND scope_id = $4;

-- name: UpdateAIDailySummaryPendingByID :one
UPDATE ai_daily_summaries
SET
    run_id = COALESCE($3, run_id),
    scope_name = COALESCE($4, scope_name),
    status = $5,
    previous_summary_id = COALESCE($6, previous_summary_id),
    prompt_version = $7,
    model = $8,
    updated_at = NOW()
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: CompleteAIDailySummary :one
UPDATE ai_daily_summaries
SET
    status = $3,
    health_score = $4,
    severity = $5,
    data_richness = $6,
    summary_depth = $7,
    headline = $8,
    executive_summary = $9,
    summary_json = $10,
    input_context_json = $11,
    linked_alert_ids = $12,
    linked_recommendation_ids = $13,
    estimated_input_tokens = $14,
    estimated_output_tokens = $15,
    estimated_cost = $16,
    validation_errors = $17,
    error_message = $18,
    updated_at = NOW()
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: FailAIDailySummary :one
UPDATE ai_daily_summaries
SET
    status = $3,
    error_message = $4,
    validation_errors = $5,
    updated_at = NOW()
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: GetPreviousAccountAIDailySummary :one
SELECT *
FROM ai_daily_summaries
WHERE seller_account_id = $1
  AND summary_type = 'account_daily_summary'
  AND scope_type = 'account'
  AND scope_id = ''
  AND metric_date < $2
  AND status = 'completed'
ORDER BY metric_date DESC
LIMIT 1;

-- name: GetPreviousCategoryAIDailySummary :one
SELECT *
FROM ai_daily_summaries
WHERE seller_account_id = $1
  AND summary_type = 'category_daily_summary'
  AND scope_type = 'category'
  AND scope_id = $2
  AND metric_date < $3
  AND status = 'completed'
ORDER BY metric_date DESC
LIMIT 1;

-- name: ListAIDailySummariesForSellerMetricDate :many
SELECT *
FROM ai_daily_summaries
WHERE seller_account_id = $1
  AND metric_date = $2
ORDER BY summary_type ASC, scope_id ASC;

-- name: GetLatestAIDailySummaryRunForSeller :one
SELECT *
FROM ai_daily_summary_runs
WHERE seller_account_id = $1
  AND status IN ('completed', 'partial')
ORDER BY metric_date DESC, completed_at DESC NULLS LAST, updated_at DESC
LIMIT 1;

-- name: ListAIDailySummariesForLatestRun :many
SELECT s.*
FROM ai_daily_summaries s
JOIN ai_daily_summary_runs r ON r.id = s.run_id
WHERE r.id = $1
  AND s.seller_account_id = $2
ORDER BY s.summary_type ASC, s.scope_id ASC;

-- name: GetLatestAIDailySummaryMetricDate :one
SELECT MAX(metric_date)::date AS latest_metric_date
FROM ai_daily_summaries
WHERE seller_account_id = $1
  AND status IN ('completed', 'not_enough_data');

-- name: GetLatestAIDailySummaryRunAnyStatus :one
SELECT *
FROM ai_daily_summary_runs
WHERE seller_account_id = $1
ORDER BY metric_date DESC, created_at DESC
LIMIT 1;

-- name: ClearAIDailySummaryRunAccountFK :exec
UPDATE ai_daily_summary_runs
SET account_summary_id = NULL,
    updated_at = NOW()
WHERE seller_account_id = $1
  AND metric_date = $2;

-- name: DeleteAIDailySummariesBySellerMetricDate :exec
DELETE FROM ai_daily_summaries
WHERE seller_account_id = $1
  AND metric_date = $2;

-- name: DeleteAIDailySummaryRunsBySellerMetricDate :exec
DELETE FROM ai_daily_summary_runs
WHERE seller_account_id = $1
  AND metric_date = $2;
