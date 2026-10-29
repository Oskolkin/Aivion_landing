-- name: CreateChatSession :one
INSERT INTO chat_sessions (
    seller_account_id,
    user_id,
    title,
    action_key
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: GetChatSessionByActionKey :one
SELECT *
FROM chat_sessions
WHERE seller_account_id = $1
  AND action_key = $2
  AND deleted_at IS NULL
ORDER BY
    CASE WHEN status = 'active' THEN 0 ELSE 1 END,
    COALESCE(last_message_at, updated_at) DESC,
    id DESC
LIMIT 1;

-- name: SoftDeleteChatSession :one
UPDATE chat_sessions
SET
    deleted_at = NOW(),
    status = 'archived',
    updated_at = NOW()
WHERE seller_account_id = $1
  AND id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: ListDuplicateActionChatSessions :many
SELECT
    seller_account_id,
    action_key,
    COUNT(*)::bigint AS sessions_count,
    ARRAY_AGG(id ORDER BY created_at) AS session_ids
FROM chat_sessions
WHERE action_key IS NOT NULL
  AND TRIM(action_key) <> ''
  AND deleted_at IS NULL
GROUP BY seller_account_id, action_key
HAVING COUNT(*) > 1;

-- name: CountChatMessagesBySessionID :one
SELECT COUNT(*)::bigint
FROM chat_messages
WHERE seller_account_id = $1
  AND session_id = $2;

-- name: MoveChatMessagesToSession :exec
UPDATE chat_messages
SET session_id = $3
WHERE seller_account_id = $1
  AND session_id = $2;

-- name: MoveChatAttachmentsToSession :exec
UPDATE chat_attachments
SET chat_session_id = $3
WHERE seller_account_id = $1
  AND chat_session_id = $2;

-- name: MoveChatTracesToSession :exec
UPDATE chat_traces
SET session_id = $3
WHERE seller_account_id = $1
  AND session_id = $2;

-- name: ReactivateChatSession :one
UPDATE chat_sessions
SET
    status = 'active',
    updated_at = NOW()
WHERE seller_account_id = $1
  AND id = $2
RETURNING *;

-- name: GetChatSessionByID :one
SELECT *
FROM chat_sessions
WHERE seller_account_id = $1
  AND id = $2;

-- name: ListChatSessionsBySellerAccountID :many
SELECT *
FROM chat_sessions
WHERE seller_account_id = $1
  AND deleted_at IS NULL
ORDER BY COALESCE(last_message_at, created_at) DESC, id DESC
LIMIT $2
OFFSET $3;

-- name: ListActiveChatSessionsBySellerAccountID :many
SELECT *
FROM chat_sessions
WHERE seller_account_id = $1
  AND status = 'active'
  AND deleted_at IS NULL
ORDER BY COALESCE(last_message_at, created_at) DESC, id DESC
LIMIT $2
OFFSET $3;

-- name: ArchiveChatSession :one
UPDATE chat_sessions
SET
    status = 'archived',
    updated_at = NOW()
WHERE seller_account_id = $1
  AND id = $2
RETURNING *;

-- name: TouchChatSession :one
UPDATE chat_sessions
SET
    last_message_at = NOW(),
    updated_at = NOW()
WHERE seller_account_id = $1
  AND id = $2
RETURNING *;

-- name: UpdateChatSessionTitle :one
UPDATE chat_sessions
SET
    title = $3,
    updated_at = NOW()
WHERE seller_account_id = $1
  AND id = $2
RETURNING *;

-- name: CreateChatMessage :one
INSERT INTO chat_messages (
    session_id,
    seller_account_id,
    role,
    content,
    message_type,
    client_request_id
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetChatMessageByClientRequestID :one
SELECT *
FROM chat_messages
WHERE seller_account_id = $1
  AND client_request_id = $2;

-- name: GetChatMessageByID :one
SELECT *
FROM chat_messages
WHERE seller_account_id = $1
  AND id = $2;

-- name: ListChatMessagesBySessionID :many
SELECT *
FROM chat_messages
WHERE seller_account_id = $1
  AND session_id = $2
ORDER BY created_at ASC, id ASC
LIMIT $3
OFFSET $4;

-- name: ListRecentChatMessagesBySessionID :many
SELECT *
FROM chat_messages
WHERE seller_account_id = $1
  AND session_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: CreateChatTrace :one
INSERT INTO chat_traces (
    session_id,
    user_message_id,
    seller_account_id,
    planner_prompt_version,
    answer_prompt_version,
    planner_model,
    answer_model,
    status,
    started_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, 'running', NOW()
)
RETURNING *;

-- name: CompleteChatTrace :one
UPDATE chat_traces
SET
    assistant_message_id = $3,
    detected_intent = $4,
    tool_plan_payload = $5,
    validated_tool_plan_payload = $6,
    tool_results_payload = $7,
    fact_context_payload = $8,
    raw_planner_response = $9,
    raw_answer_response = $10,
    answer_validation_payload = $11,
    input_tokens = $12,
    output_tokens = $13,
    estimated_cost = $14,
    status = 'completed',
    error_message = NULL,
    finished_at = NOW()
WHERE seller_account_id = $1
  AND id = $2
RETURNING *;

-- name: FailChatTrace :one
UPDATE chat_traces
SET
    detected_intent = $3,
    tool_plan_payload = $4,
    validated_tool_plan_payload = $5,
    tool_results_payload = $6,
    fact_context_payload = $7,
    raw_planner_response = $8,
    raw_answer_response = $9,
    answer_validation_payload = $10,
    input_tokens = $11,
    output_tokens = $12,
    estimated_cost = $13,
    error_message = $14,
    status = 'failed',
    finished_at = NOW()
WHERE seller_account_id = $1
  AND id = $2
RETURNING *;

-- name: GetChatTraceByID :one
SELECT *
FROM chat_traces
WHERE seller_account_id = $1
  AND id = $2;

-- name: GetLatestChatTraceBySessionID :one
SELECT *
FROM chat_traces
WHERE seller_account_id = $1
  AND session_id = $2
ORDER BY started_at DESC, id DESC
LIMIT 1;

-- name: ListChatTracesBySessionID :many
SELECT *
FROM chat_traces
WHERE seller_account_id = $1
  AND session_id = $2
ORDER BY started_at DESC, id DESC
LIMIT $3
OFFSET $4;

-- name: CreateChatFeedback :one
INSERT INTO chat_feedback (
    session_id,
    message_id,
    seller_account_id,
    rating,
    comment
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (seller_account_id, message_id)
DO UPDATE SET
    rating = EXCLUDED.rating,
    comment = EXCLUDED.comment,
    created_at = NOW()
RETURNING *;

-- name: GetChatFeedbackByMessageID :one
SELECT *
FROM chat_feedback
WHERE seller_account_id = $1
  AND message_id = $2;

-- name: ListChatFeedbackBySellerAccountID :many
SELECT *
FROM chat_feedback
WHERE seller_account_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2
OFFSET $3;

-- name: CountChatSessionsBySellerAccountID :one
SELECT COUNT(*)::bigint
FROM chat_sessions
WHERE seller_account_id = $1;

-- name: CountChatMessagesBySellerAccountID :one
SELECT COUNT(*)::bigint
FROM chat_messages
WHERE seller_account_id = $1;

-- name: CountChatTracesBySellerAccountID :one
SELECT COUNT(*)::bigint
FROM chat_traces
WHERE seller_account_id = $1;

-- name: CountCompletedChatTracesBySellerAccountID :one
SELECT COUNT(*)::bigint
FROM chat_traces
WHERE seller_account_id = $1
  AND status = 'completed';
