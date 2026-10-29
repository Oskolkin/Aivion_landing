-- name: AdminListClients :many
SELECT
    sa.id AS seller_account_id,
    sa.name AS seller_name,
    sa.status AS seller_status,
    sa.created_at AS seller_created_at,
    sa.updated_at AS seller_updated_at,
    u.id AS owner_user_id,
    u.email AS owner_email,
    oc.status AS connection_status,
    oc.last_check_at AS connection_last_check_at,
    oc.last_error AS connection_last_error,
    sj.status AS latest_sync_status,
    sj.started_at AS latest_sync_started_at,
    sj.finished_at AS latest_sync_finished_at,
    COALESCE((
        SELECT COUNT(*)
        FROM alerts a
        WHERE a.seller_account_id = sa.id
          AND a.status = 'open'
    ), 0)::bigint AS open_alerts_count,
    COALESCE((
        SELECT COUNT(*)
        FROM recommendations r
        WHERE r.seller_account_id = sa.id
          AND r.status = 'open'
    ), 0)::bigint AS open_recommendations_count,
    rr.status AS latest_recommendation_run_status,
    ct.status AS latest_chat_trace_status,
    sbs.status AS billing_status
FROM seller_accounts sa
JOIN users u ON u.id = sa.user_id
LEFT JOIN ozon_connections oc ON oc.seller_account_id = sa.id
LEFT JOIN LATERAL (
    SELECT status, started_at, finished_at
    FROM sync_jobs
    WHERE seller_account_id = sa.id
    ORDER BY created_at DESC, id DESC
    LIMIT 1
) sj ON TRUE
LEFT JOIN LATERAL (
    SELECT status
    FROM recommendation_runs
    WHERE seller_account_id = sa.id
    ORDER BY started_at DESC, id DESC
    LIMIT 1
) rr ON TRUE
LEFT JOIN LATERAL (
    SELECT status
    FROM chat_traces
    WHERE seller_account_id = sa.id
    ORDER BY started_at DESC, id DESC
    LIMIT 1
) ct ON TRUE
LEFT JOIN seller_billing_state sbs ON sbs.seller_account_id = sa.id
WHERE (
    sqlc.narg(search)::text IS NULL
    OR sa.name ILIKE '%' || sqlc.narg(search)::text || '%'
    OR u.email ILIKE '%' || sqlc.narg(search)::text || '%'
)
  AND (
    sqlc.narg(seller_status)::text IS NULL
    OR sa.status = sqlc.narg(seller_status)::text
)
  AND (
    sqlc.narg(connection_status)::text IS NULL
    OR oc.status = sqlc.narg(connection_status)::text
)
  AND (
    sqlc.narg(billing_status)::text IS NULL
    OR sbs.status = sqlc.narg(billing_status)::text
)
ORDER BY sa.created_at DESC, sa.id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: AdminGetClientOverview :one
SELECT
    sa.id AS seller_account_id,
    sa.name AS seller_name,
    sa.status AS seller_status,
    sa.user_id AS owner_user_id,
    u.email AS owner_email,
    sa.created_at,
    sa.updated_at
FROM seller_accounts sa
JOIN users u ON u.id = sa.user_id
WHERE sa.id = $1;

-- name: AdminListClientConnections :many
SELECT
    'ozon'::text AS provider,
    oc.status AS connection_status,
    oc.last_check_at,
    oc.last_check_result,
    oc.last_error,
    oc.updated_at,
    oc.performance_status AS performance_connection_status,
    (oc.performance_token_encrypted IS NOT NULL
        AND length(trim(oc.performance_token_encrypted)) > 0)::boolean AS performance_token_set,
    oc.performance_last_check_at,
    oc.performance_last_check_result,
    oc.performance_last_error
FROM ozon_connections oc
WHERE oc.seller_account_id = $1
ORDER BY oc.updated_at DESC;

-- name: AdminListSyncJobs :many
SELECT
    id,
    seller_account_id,
    type,
    status,
    started_at,
    finished_at,
    error_message,
    created_at
FROM sync_jobs
WHERE seller_account_id = $1
  AND (
    sqlc.narg(status)::text IS NULL
    OR status = sqlc.narg(status)::text
)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: AdminListImportJobs :many
SELECT
    id,
    seller_account_id,
    sync_job_id,
    domain,
    status,
    source_cursor,
    records_received,
    records_imported,
    records_failed,
    started_at,
    finished_at,
    error_message,
    created_at
FROM import_jobs
WHERE seller_account_id = $1
  AND (
    sqlc.narg(status)::text IS NULL
    OR status = sqlc.narg(status)::text
)
  AND (
    sqlc.narg(domain)::text IS NULL
    OR domain = sqlc.narg(domain)::text
)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: AdminListImportErrors :many
SELECT
    id,
    seller_account_id,
    sync_job_id,
    domain,
    status,
    error_message,
    records_failed,
    started_at,
    finished_at,
    created_at
FROM import_jobs
WHERE seller_account_id = $1
  AND error_message IS NOT NULL
  AND error_message <> ''
  AND (
    sqlc.narg(domain)::text IS NULL
    OR domain = sqlc.narg(domain)::text
)
  AND (
    status = 'failed'
    OR sqlc.narg(status)::text IS NULL
    OR status = sqlc.narg(status)::text
)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: AdminListSyncCursors :many
SELECT
    id,
    seller_account_id,
    domain,
    cursor_type,
    cursor_value,
    updated_at
FROM sync_cursors
WHERE seller_account_id = $1
  AND (
    sqlc.narg(domain)::text IS NULL
    OR domain = sqlc.narg(domain)::text
)
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: AdminListChatTracesBySeller :many
SELECT
    id,
    session_id,
    user_message_id,
    assistant_message_id,
    seller_account_id,
    planner_prompt_version,
    answer_prompt_version,
    planner_model,
    answer_model,
    detected_intent,
    tool_plan_payload,
    validated_tool_plan_payload,
    tool_results_payload,
    fact_context_payload,
    raw_planner_response,
    raw_answer_response,
    answer_validation_payload,
    input_tokens,
    output_tokens,
    estimated_cost,
    status,
    error_message,
    started_at,
    finished_at,
    created_at
FROM chat_traces
WHERE seller_account_id = $1
  AND (
    sqlc.narg(status)::text IS NULL
    OR status = sqlc.narg(status)::text
)
  AND (
    sqlc.narg(detected_intent)::text IS NULL
    OR detected_intent = sqlc.narg(detected_intent)::text
)
  AND (
    sqlc.narg(session_id)::bigint IS NULL
    OR session_id = sqlc.narg(session_id)::bigint
)
ORDER BY started_at DESC, created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: AdminGetChatSessionByID :one
SELECT
    id,
    seller_account_id,
    title,
    status,
    created_at,
    updated_at,
    last_message_at
FROM chat_sessions
WHERE seller_account_id = $1
  AND id = $2;

-- name: AdminListChatSessionsBySeller :many
SELECT
    id,
    seller_account_id,
    title,
    status,
    created_at,
    updated_at,
    last_message_at
FROM chat_sessions
WHERE seller_account_id = $1
  AND (
    sqlc.narg(status)::text IS NULL
    OR status = sqlc.narg(status)::text
)
ORDER BY last_message_at DESC NULLS LAST, updated_at DESC, id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: AdminListChatMessagesBySession :many
SELECT
    id,
    session_id,
    seller_account_id,
    role,
    content,
    message_type,
    created_at
FROM chat_messages
WHERE seller_account_id = $1
  AND session_id = $2
ORDER BY created_at ASC, id ASC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: AdminGetRecommendationRunByID :one
SELECT
    id,
    seller_account_id,
    run_type,
    status,
    as_of_date,
    ai_model,
    ai_prompt_version,
    input_tokens,
    output_tokens,
    estimated_cost,
    generated_recommendations_count,
    accepted_recommendations_count,
    error_message,
    started_at,
    finished_at,
    created_at
FROM recommendation_runs
WHERE seller_account_id = $1
  AND id = $2;

-- name: AdminListRecommendationRuns :many
SELECT
    id,
    seller_account_id,
    run_type,
    status,
    as_of_date,
    ai_model,
    ai_prompt_version,
    input_tokens,
    output_tokens,
    estimated_cost,
    generated_recommendations_count,
    accepted_recommendations_count,
    error_message,
    started_at,
    finished_at,
    created_at
FROM recommendation_runs
WHERE seller_account_id = $1
  AND (
    sqlc.narg(status)::text IS NULL
    OR status = sqlc.narg(status)::text
)
  AND (
    sqlc.narg(run_type)::text IS NULL
    OR run_type = sqlc.narg(run_type)::text
)
ORDER BY started_at DESC, created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: AdminListChatFeedback :many
SELECT
    cf.id,
    cf.seller_account_id,
    sa.name AS seller_name,
    cf.session_id,
    cf.message_id,
    cf.rating,
    cf.comment,
    cf.created_at,
    cm.role AS message_role,
    cm.message_type,
    cm.content AS message_content,
    cs.title AS session_title,
    ct.id AS trace_id
FROM chat_feedback cf
JOIN chat_messages cm ON cm.id = cf.message_id AND cm.seller_account_id = cf.seller_account_id
JOIN chat_sessions cs ON cs.id = cf.session_id AND cs.seller_account_id = cf.seller_account_id
JOIN seller_accounts sa ON sa.id = cf.seller_account_id
LEFT JOIN chat_traces ct ON ct.seller_account_id = cf.seller_account_id AND ct.assistant_message_id = cf.message_id
WHERE (
    sqlc.narg(seller_account_id)::bigint IS NULL
    OR cf.seller_account_id = sqlc.narg(seller_account_id)::bigint
)
  AND (
    sqlc.narg(rating)::text IS NULL
    OR cf.rating = sqlc.narg(rating)::text
)
ORDER BY cf.created_at DESC, cf.id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: AdminListRecommendationFeedbackBySeller :many
SELECT
    rf.id,
    rf.seller_account_id,
    rf.recommendation_id,
    rf.rating,
    rf.comment,
    rf.created_at AS feedback_created_at,
    r.recommendation_type,
    r.title,
    r.priority_level,
    r.confidence_level,
    r.status AS recommendation_status,
    r.entity_type,
    r.entity_id,
    r.entity_sku,
    r.entity_offer_id,
    r.created_at AS recommendation_created_at
FROM recommendation_feedback rf
JOIN recommendations r ON r.id = rf.recommendation_id AND r.seller_account_id = rf.seller_account_id
WHERE rf.seller_account_id = $1
  AND (
    sqlc.narg(rating)::text IS NULL
    OR rf.rating = sqlc.narg(rating)::text
)
  AND (
    sqlc.narg(recommendation_status)::text IS NULL
    OR r.status = sqlc.narg(recommendation_status)::text
)
ORDER BY rf.created_at DESC, rf.id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: AdminGetRecommendationProxyFeedbackCounts :one
SELECT
    COUNT(*) FILTER (WHERE status = 'accepted')::bigint AS accepted_count,
    COUNT(*) FILTER (WHERE status = 'dismissed')::bigint AS dismissed_count,
    COUNT(*) FILTER (WHERE status = 'resolved')::bigint AS resolved_count
FROM recommendations
WHERE seller_account_id = $1;
