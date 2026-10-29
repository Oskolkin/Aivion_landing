-- name: CreateAdminActionLog :one
INSERT INTO admin_action_logs (
    admin_user_id,
    admin_email,
    seller_account_id,
    action_type,
    target_type,
    target_id,
    request_payload,
    status
) VALUES (
    sqlc.narg(admin_user_id)::bigint,
    sqlc.arg(admin_email)::text,
    sqlc.arg(seller_account_id)::bigint,
    sqlc.arg(action_type)::text,
    sqlc.narg(target_type)::text,
    sqlc.narg(target_id)::bigint,
    sqlc.arg(request_payload)::jsonb,
    COALESCE(sqlc.narg(status)::text, 'running')
)
RETURNING *;

-- name: CompleteAdminActionLog :one
UPDATE admin_action_logs
SET
    status = 'completed',
    result_payload = $2,
    error_message = NULL,
    finished_at = NOW()
WHERE id = $1
RETURNING *;

-- name: FailAdminActionLog :one
UPDATE admin_action_logs
SET
    status = 'failed',
    result_payload = $2,
    error_message = $3,
    finished_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetAdminActionLogByID :one
SELECT *
FROM admin_action_logs
WHERE id = $1;

-- name: CountAdminActionLogsBySellerAccountID :one
SELECT COUNT(*)::bigint
FROM admin_action_logs
WHERE seller_account_id = $1;

-- name: ListAdminActionLogs :many
SELECT *
FROM admin_action_logs
WHERE (
    sqlc.narg(seller_account_id)::bigint IS NULL
    OR seller_account_id = sqlc.narg(seller_account_id)::bigint
)
  AND (
    sqlc.narg(action_type)::text IS NULL
    OR action_type = sqlc.narg(action_type)::text
)
  AND (
    sqlc.narg(status)::text IS NULL
    OR status = sqlc.narg(status)::text
)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: GetSellerBillingState :one
SELECT *
FROM seller_billing_state
WHERE seller_account_id = $1;

-- name: UpsertSellerBillingState :one
INSERT INTO seller_billing_state (
    seller_account_id,
    plan_code,
    status,
    trial_ends_at,
    current_period_start,
    current_period_end,
    ai_tokens_limit_month,
    ai_tokens_used_month,
    estimated_ai_cost_month,
    notes
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (seller_account_id)
DO UPDATE SET
    plan_code = EXCLUDED.plan_code,
    status = EXCLUDED.status,
    trial_ends_at = EXCLUDED.trial_ends_at,
    current_period_start = EXCLUDED.current_period_start,
    current_period_end = EXCLUDED.current_period_end,
    ai_tokens_limit_month = EXCLUDED.ai_tokens_limit_month,
    ai_tokens_used_month = EXCLUDED.ai_tokens_used_month,
    estimated_ai_cost_month = EXCLUDED.estimated_ai_cost_month,
    notes = EXCLUDED.notes,
    updated_at = NOW()
RETURNING *;

-- name: ListSellerBillingStates :many
SELECT *
FROM seller_billing_state
WHERE (
    sqlc.narg(status)::text IS NULL
    OR status = sqlc.narg(status)::text
)
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: AdminPeekRecommendationForAudit :one
SELECT
    id,
    COALESCE(ai_model, '') AS ai_model,
    COALESCE(ai_prompt_version, '') AS ai_prompt_version
FROM recommendations
WHERE id = $1 AND seller_account_id = $2;

-- name: AdminPeekChatTraceForAudit :one
SELECT
    id,
    session_id,
    user_message_id,
    assistant_message_id,
    planner_model,
    answer_model,
    planner_prompt_version,
    answer_prompt_version,
    status
FROM chat_traces
WHERE id = $1 AND seller_account_id = $2;

-- name: AdminPeekRecommendationRunForAudit :one
SELECT
    id,
    run_type,
    status,
    COALESCE(ai_model, '') AS ai_model,
    COALESCE(ai_prompt_version, '') AS ai_prompt_version
FROM recommendation_runs
WHERE id = $1 AND seller_account_id = $2;

-- name: CreateRecommendationRunDiagnostic :one
INSERT INTO recommendation_run_diagnostics (
    recommendation_run_id,
    seller_account_id,
    openai_request_id,
    ai_model,
    prompt_version,
    context_payload_summary,
    raw_openai_response,
    validation_result_payload,
    rejected_items_payload,
    error_stage,
    error_message,
    input_tokens,
    output_tokens,
    estimated_cost
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;

-- name: GetRecommendationRunDiagnosticByID :one
SELECT *
FROM recommendation_run_diagnostics
WHERE seller_account_id = $1
  AND id = $2;

-- name: ListRecommendationRunDiagnosticsBySeller :many
SELECT *
FROM recommendation_run_diagnostics
WHERE seller_account_id = $1
  AND (
    sqlc.narg(recommendation_run_id)::bigint IS NULL
    OR recommendation_run_id = sqlc.narg(recommendation_run_id)::bigint
)
  AND (
    sqlc.narg(error_stage)::text IS NULL
    OR error_stage = sqlc.narg(error_stage)::text
)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: ListRecommendationRunDiagnosticsByRun :many
SELECT *
FROM recommendation_run_diagnostics
WHERE seller_account_id = $1
  AND recommendation_run_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3
OFFSET $4;
