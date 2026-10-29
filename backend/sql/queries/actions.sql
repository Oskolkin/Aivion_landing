-- name: CreateActionGenerationRun :one
INSERT INTO action_generation_runs (
    seller_account_id,
    metric_date,
    status
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: UpdateActionGenerationRun :one
UPDATE action_generation_runs
SET
    status = $2,
    context_hash = $3,
    final_status = $4,
    accepted_count = $5,
    repaired_count = $6,
    dropped_count = $7,
    fallback_count = $8,
    ai_focus_json = $9,
    top_action_keys_json = $10,
    data_quality_notes_json = $11,
    updated_at = now()
WHERE action_generation_runs.id = $1
  AND NOT EXISTS (
    SELECT 1
    FROM ai_pipeline_runs p
    WHERE p.actions_run_id = action_generation_runs.id
      AND p.published_at IS NOT NULL
  )
RETURNING *;

-- name: CreateActionGenerationCategoryRun :one
INSERT INTO action_generation_category_runs (
    run_id,
    seller_account_id,
    domain,
    status,
    model,
    input_context_json,
    raw_response_json,
    validation_json,
    repair_response_json,
    final_validation_json,
    accepted_count,
    repaired_count,
    dropped_count
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING *;

-- name: UpdateActionGenerationCategoryRun :one
UPDATE action_generation_category_runs
SET
    status = $2,
    model = $3,
    input_context_json = $4,
    raw_response_json = $5,
    validation_json = $6,
    repair_response_json = $7,
    final_validation_json = $8,
    accepted_count = $9,
    repaired_count = $10,
    dropped_count = $11,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpsertActionItem :one
INSERT INTO action_items (
    seller_account_id,
    run_id,
    action_key,
    type,
    category,
    priority,
    title,
    summary,
    reason,
    next_step,
    expected_effect,
    evidence_json,
    limitations_json,
    cta_json,
    affected_entities_json,
    entity_group_ref,
    source_candidate_ids_json,
    evidence_fact_ids_json,
    payload_json,
    metric_date
) SELECT
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
WHERE (
    $2::bigint IS NULL
    OR NOT EXISTS (
        SELECT 1
        FROM ai_pipeline_runs p
        WHERE p.actions_run_id = $2::bigint
          AND p.published_at IS NOT NULL
    )
)
ON CONFLICT (run_id, action_key) WHERE run_id IS NOT NULL
DO UPDATE SET
    run_id = EXCLUDED.run_id,
    type = EXCLUDED.type,
    category = EXCLUDED.category,
    priority = EXCLUDED.priority,
    title = EXCLUDED.title,
    summary = EXCLUDED.summary,
    reason = EXCLUDED.reason,
    next_step = EXCLUDED.next_step,
    expected_effect = EXCLUDED.expected_effect,
    evidence_json = EXCLUDED.evidence_json,
    limitations_json = EXCLUDED.limitations_json,
    cta_json = EXCLUDED.cta_json,
    affected_entities_json = EXCLUDED.affected_entities_json,
    entity_group_ref = EXCLUDED.entity_group_ref,
    source_candidate_ids_json = EXCLUDED.source_candidate_ids_json,
    evidence_fact_ids_json = EXCLUDED.evidence_fact_ids_json,
    payload_json = EXCLUDED.payload_json,
    metric_date = EXCLUDED.metric_date,
    updated_at = now()
WHERE NOT EXISTS (
    SELECT 1
    FROM ai_pipeline_runs p
    WHERE p.actions_run_id = action_items.run_id
      AND p.published_at IS NOT NULL
)
RETURNING *;

-- name: DeleteStaleActionItemsForRun :exec
DELETE FROM action_items
WHERE action_items.run_id = $1
  AND action_items.seller_account_id = $2
  AND action_items.action_key <> ALL($3::text[])
  AND NOT EXISTS (
    SELECT 1
    FROM ai_pipeline_runs p
    WHERE p.actions_run_id = action_items.run_id
      AND p.published_at IS NOT NULL
  );

-- name: DeleteStaleActionItemsForDate :exec
DELETE FROM action_items
WHERE action_items.seller_account_id = $1
  AND action_items.metric_date = $2
  AND action_items.action_key <> ALL($3::text[])
  AND action_items.run_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM ai_pipeline_runs p
    WHERE p.actions_run_id = action_items.run_id
      AND p.published_at IS NOT NULL
  );

-- name: GetLatestActionGenerationRun :one
SELECT *
FROM action_generation_runs
WHERE seller_account_id = $1
ORDER BY metric_date DESC NULLS LAST, created_at DESC
LIMIT 1;

-- name: GetLatestActionMetricDate :one
SELECT MAX(metric_date)::date AS metric_date
FROM action_items
WHERE seller_account_id = $1;

-- name: ListActionItemsBySeller :many
SELECT
    ai.*,
    COALESCE(aus.status, 'open')::text AS user_status,
    aus.checked_at,
    aus.snoozed_until,
    aus.dismissed_at
FROM action_items ai
LEFT JOIN action_user_states aus
    ON aus.seller_account_id = ai.seller_account_id
    AND aus.action_key = ai.action_key
WHERE ai.seller_account_id = sqlc.arg(seller_account_id)
  AND (
    sqlc.narg(metric_date)::date IS NULL
    OR ai.metric_date = sqlc.narg(metric_date)::date
  )
  AND (
    sqlc.narg(category)::text IS NULL
    OR ai.category = sqlc.narg(category)::text
  )
  AND (
    sqlc.narg(priority)::text IS NULL
    OR ai.priority = sqlc.narg(priority)::text
  )
  AND (
    sqlc.narg(status)::text IS NULL
    OR sqlc.narg(status)::text = 'all'
    OR (
      sqlc.narg(status)::text = 'open'
      AND COALESCE(aus.status, 'open') = 'open'
    )
    OR (
      sqlc.narg(status)::text = 'checked'
      AND aus.status = 'checked'
    )
    OR (
      sqlc.narg(status)::text = 'snoozed'
      AND aus.status = 'snoozed'
    )
    OR (
      sqlc.narg(status)::text = 'dismissed'
      AND aus.status = 'dismissed'
    )
  )
ORDER BY
    CASE ai.priority
        WHEN 'critical' THEN 1
        WHEN 'important' THEN 2
        WHEN 'watch' THEN 3
        ELSE 4
    END,
    ai.created_at DESC
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);

-- name: GetActionItemByKey :one
SELECT
    ai.*,
    COALESCE(aus.status, 'open')::text AS user_status,
    aus.checked_at,
    aus.snoozed_until,
    aus.dismissed_at
FROM action_items ai
LEFT JOIN action_user_states aus
    ON aus.seller_account_id = ai.seller_account_id
    AND aus.action_key = ai.action_key
WHERE ai.seller_account_id = $1
  AND ai.action_key = $2
  AND (
    sqlc.narg(run_id)::bigint IS NULL
    OR ai.run_id = sqlc.narg(run_id)::bigint
  );

-- name: GetActionItemByIDForSeller :one
SELECT
    ai.*,
    COALESCE(aus.status, 'open')::text AS user_status,
    aus.checked_at,
    aus.snoozed_until,
    aus.dismissed_at
FROM action_items ai
LEFT JOIN action_user_states aus
    ON aus.seller_account_id = ai.seller_account_id
    AND aus.action_key = ai.action_key
WHERE ai.seller_account_id = $1
  AND ai.id = $2;

-- name: GetActionItemByIDAndRunIDForSeller :one
SELECT
    ai.*,
    COALESCE(aus.status, 'open')::text AS user_status,
    aus.checked_at,
    aus.snoozed_until,
    aus.dismissed_at
FROM action_items ai
LEFT JOIN action_user_states aus
    ON aus.seller_account_id = ai.seller_account_id
    AND aus.action_key = ai.action_key
WHERE ai.seller_account_id = $1
  AND ai.id = $2
  AND ai.run_id = $3;

-- name: GetActionItemByIDForCurrentPublishedRun :one
SELECT
    ai.*,
    COALESCE(aus.status, 'open')::text AS user_status,
    aus.checked_at,
    aus.snoozed_until,
    aus.dismissed_at
FROM action_items ai
INNER JOIN action_generation_runs agr
    ON agr.id = ai.run_id
    AND agr.seller_account_id = ai.seller_account_id
LEFT JOIN action_user_states aus
    ON aus.seller_account_id = ai.seller_account_id
    AND aus.action_key = ai.action_key
WHERE ai.seller_account_id = $1
  AND ai.id = $2
  AND agr.status = 'completed'
  AND COALESCE(agr.final_status, 'completed') IN (
    'completed',
    'completed_repaired',
    'completed_partial',
    'completed_after_retry',
    'empty'
  )
  AND agr.id = (
    SELECT r.id
    FROM action_generation_runs r
    WHERE r.seller_account_id = $1
      AND r.status = 'completed'
      AND COALESCE(r.final_status, 'completed') IN (
        'completed',
        'completed_repaired',
        'completed_partial',
        'completed_after_retry',
        'empty'
      )
    ORDER BY r.metric_date DESC NULLS LAST, r.created_at DESC, r.id DESC
    LIMIT 1
  );

-- name: GetHistoricalActionItemByIDForSeller :one
SELECT
    ai.*,
    COALESCE(aus.status, 'open')::text AS user_status,
    aus.checked_at,
    aus.snoozed_until,
    aus.dismissed_at
FROM action_items ai
LEFT JOIN action_user_states aus
    ON aus.seller_account_id = ai.seller_account_id
    AND aus.action_key = ai.action_key
WHERE ai.seller_account_id = $1
  AND ai.id = $2;

-- name: GetDailyAccountMetricBySellerAndDate :one
SELECT *
FROM daily_account_metrics
WHERE seller_account_id = $1
  AND metric_date = $2;

-- name: CountAdCampaignsBySeller :one
SELECT COUNT(*)::bigint AS campaign_count
FROM ad_campaigns
WHERE seller_account_id = $1;

-- name: SumAdSpendBySellerAndDateRange :one
SELECT COALESCE(SUM(spend), 0)::numeric(18,2) AS spend_total
FROM ad_metrics_daily
WHERE seller_account_id = $1
  AND metric_date >= $2
  AND metric_date <= $3;

-- name: UpsertActionEntityGroup :one
INSERT INTO action_entity_groups (
    seller_account_id,
    metric_date,
    group_key,
    entity_type,
    entity_ids_json,
    summary_json,
    action_generation_run_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (action_generation_run_id, group_key)
DO UPDATE SET
    metric_date = EXCLUDED.metric_date,
    entity_type = EXCLUDED.entity_type,
    entity_ids_json = EXCLUDED.entity_ids_json,
    summary_json = EXCLUDED.summary_json
WHERE NOT EXISTS (
    SELECT 1
    FROM action_generation_runs r
    INNER JOIN ai_pipeline_runs p ON p.id = r.ai_pipeline_run_id
    WHERE r.id = action_entity_groups.action_generation_run_id
      AND p.published_at IS NOT NULL
)
RETURNING *;

-- name: UpsertActionUserState :exec
INSERT INTO action_user_states (
    seller_account_id,
    action_key,
    status,
    checked_at,
    snoozed_until,
    dismissed_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, now()
)
ON CONFLICT (seller_account_id, action_key)
DO UPDATE SET
    status = EXCLUDED.status,
    checked_at = EXCLUDED.checked_at,
    snoozed_until = EXCLUDED.snoozed_until,
    dismissed_at = EXCLUDED.dismissed_at,
    updated_at = now();

-- name: ListActionGenerationRunsForAdmin :many
SELECT *
FROM action_generation_runs
WHERE (
    sqlc.narg(seller_account_id)::bigint IS NULL
    OR seller_account_id = sqlc.narg(seller_account_id)::bigint
)
  AND (
    sqlc.narg(status)::text IS NULL
    OR status = sqlc.narg(status)::text
)
  AND (
    sqlc.narg(final_status)::text IS NULL
    OR final_status = sqlc.narg(final_status)::text
)
  AND (
    sqlc.narg(metric_date)::date IS NULL
    OR metric_date = sqlc.narg(metric_date)::date
)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);

-- name: GetActionGenerationRunForAdmin :one
SELECT *
FROM action_generation_runs
WHERE id = $1;

-- name: GetActionGenerationRunForUpdate :one
SELECT *
FROM action_generation_runs
WHERE id = $1
FOR UPDATE;

-- name: ListActionGenerationCategoryRunsForAdmin :many
SELECT *
FROM action_generation_category_runs
WHERE run_id = $1
ORDER BY created_at ASC, id ASC;

-- name: GetActionEntityGroup :one
SELECT
  id,
  seller_account_id,
  metric_date,
  group_key,
  entity_type,
  entity_ids_json,
  summary_json,
  created_at,
  action_generation_run_id
FROM action_entity_groups
WHERE seller_account_id = $1
  AND action_generation_run_id = $2
  AND group_key = $3
LIMIT 1;

-- name: ListActionItemsByRunID :many
SELECT *
FROM action_items
WHERE run_id = $1
ORDER BY created_at ASC, id ASC;

-- name: ListActionItemsWithUserStateByRunID :many
SELECT
    ai.*,
    COALESCE(aus.status, 'open')::text AS user_status,
    aus.checked_at,
    aus.snoozed_until,
    aus.dismissed_at
FROM action_items ai
LEFT JOIN action_user_states aus
    ON aus.seller_account_id = ai.seller_account_id
    AND aus.action_key = ai.action_key
WHERE ai.run_id = $1
ORDER BY ai.created_at ASC, ai.id ASC;

-- name: GetLatestPublishableActionGenerationRun :one
SELECT *
FROM action_generation_runs
WHERE seller_account_id = $1
  AND status = 'completed'
  AND COALESCE(final_status, 'completed') IN (
    'completed',
    'completed_repaired',
    'completed_partial',
    'completed_after_retry',
    'empty'
  )
ORDER BY metric_date DESC NULLS LAST, created_at DESC, id DESC
LIMIT 1;

-- name: GetLatestPublishableActionGenerationRunForSellerAndDate :one
SELECT *
FROM action_generation_runs
WHERE seller_account_id = $1
  AND metric_date = $2
  AND status = 'completed'
  AND COALESCE(final_status, 'completed') IN (
    'completed',
    'completed_repaired',
    'completed_partial',
    'completed_after_retry',
    'empty'
  )
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetLatestCompletedActionGenerationRunForSellerAndDate :one
SELECT *
FROM action_generation_runs
WHERE seller_account_id = $1
  AND metric_date = $2
  AND status = 'completed'
  AND COALESCE(final_status, 'completed') IN ('completed', 'completed_partial', 'completed_after_retry', 'completed_repaired', 'empty')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: DeleteActionItemByRunIDAndKey :exec
DELETE FROM action_items
WHERE run_id = $1
  AND action_key = $2
  AND NOT EXISTS (
    SELECT 1
    FROM ai_pipeline_runs p
    WHERE p.actions_run_id = action_items.run_id
      AND p.published_at IS NOT NULL
  );
