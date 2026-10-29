-- name: CreateAIUsageEvent :one
INSERT INTO ai_usage_events (
    seller_account_id,
    pipeline_run_id,
    component,
    attempt,
    model,
    request_id,
    input_tokens,
    output_tokens,
    total_tokens,
    estimated_cost,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: ListAIUsageEventsByPipelineRun :many
SELECT *
FROM ai_usage_events
WHERE pipeline_run_id = $1
ORDER BY created_at ASC, id ASC;
