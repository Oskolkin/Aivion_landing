-- name: UpsertRecommendation :one
INSERT INTO recommendations (
    seller_account_id,
    source,
    recommendation_type,
    horizon,
    entity_type,
    entity_id,
    entity_sku,
    entity_offer_id,
    title,
    what_happened,
    why_it_matters,
    recommended_action,
    expected_effect,
    priority_score,
    priority_level,
    urgency,
    confidence_level,
    supporting_metrics_payload,
    constraints_payload,
    ai_model,
    ai_prompt_version,
    raw_ai_response,
    fingerprint,
    first_seen_at,
    last_seen_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, NOW(), NOW(), NOW()
)
ON CONFLICT (seller_account_id, fingerprint)
DO UPDATE SET
    source = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.source
        ELSE EXCLUDED.source
    END,
    recommendation_type = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.recommendation_type
        ELSE EXCLUDED.recommendation_type
    END,
    horizon = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.horizon
        ELSE EXCLUDED.horizon
    END,
    entity_type = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.entity_type
        ELSE EXCLUDED.entity_type
    END,
    entity_id = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.entity_id
        ELSE EXCLUDED.entity_id
    END,
    entity_sku = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.entity_sku
        ELSE EXCLUDED.entity_sku
    END,
    entity_offer_id = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.entity_offer_id
        ELSE EXCLUDED.entity_offer_id
    END,
    title = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.title
        ELSE EXCLUDED.title
    END,
    what_happened = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.what_happened
        ELSE EXCLUDED.what_happened
    END,
    why_it_matters = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.why_it_matters
        ELSE EXCLUDED.why_it_matters
    END,
    recommended_action = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.recommended_action
        ELSE EXCLUDED.recommended_action
    END,
    expected_effect = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.expected_effect
        ELSE EXCLUDED.expected_effect
    END,
    priority_score = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.priority_score
        ELSE EXCLUDED.priority_score
    END,
    priority_level = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.priority_level
        ELSE EXCLUDED.priority_level
    END,
    urgency = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.urgency
        ELSE EXCLUDED.urgency
    END,
    confidence_level = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.confidence_level
        ELSE EXCLUDED.confidence_level
    END,
    status = CASE
        WHEN recommendations.status = 'dismissed' THEN 'dismissed'
        WHEN recommendations.status = 'accepted' THEN 'accepted'
        ELSE 'open'
    END,
    supporting_metrics_payload = EXCLUDED.supporting_metrics_payload,
    constraints_payload = EXCLUDED.constraints_payload,
    ai_model = EXCLUDED.ai_model,
    ai_prompt_version = EXCLUDED.ai_prompt_version,
    raw_ai_response = EXCLUDED.raw_ai_response,
    last_seen_at = NOW(),
    resolved_at = CASE
        WHEN recommendations.status IN ('dismissed', 'accepted') THEN recommendations.resolved_at
        ELSE NULL
    END,
    updated_at = NOW()
RETURNING *;

-- name: GetRecommendationByID :one
SELECT *
FROM recommendations
WHERE id = $1
  AND seller_account_id = $2;

-- name: ListRecommendationsBySellerAccountID :many
SELECT *
FROM recommendations
WHERE seller_account_id = $1
ORDER BY last_seen_at DESC, id DESC
LIMIT $2
OFFSET $3;

-- name: ListOpenRecommendationsBySellerAccountID :many
SELECT *
FROM recommendations
WHERE seller_account_id = $1
  AND status = 'open'
ORDER BY last_seen_at DESC, id DESC
LIMIT $2
OFFSET $3;

-- name: ListRecommendationsFiltered :many
SELECT *
FROM recommendations
WHERE seller_account_id = $1
  AND (
    status = sqlc.narg(status)
    OR (
      sqlc.narg(status)::text IS NULL
      AND status IS NOT NULL
    )
  )
  AND (
    recommendation_type = sqlc.narg(recommendation_type)
    OR (
      sqlc.narg(recommendation_type)::text IS NULL
      AND recommendation_type IS NOT NULL
    )
  )
  AND (
    priority_level = sqlc.narg(priority_level)
    OR (
      sqlc.narg(priority_level)::text IS NULL
      AND priority_level IS NOT NULL
    )
  )
  AND (
    confidence_level = sqlc.narg(confidence_level)
    OR (
      sqlc.narg(confidence_level)::text IS NULL
      AND confidence_level IS NOT NULL
    )
  )
  AND (
    horizon = sqlc.narg(horizon)
    OR (
      sqlc.narg(horizon)::text IS NULL
      AND horizon IS NOT NULL
    )
  )
  AND (
    entity_type = sqlc.narg(entity_type)
    OR (
      sqlc.narg(entity_type)::text IS NULL
      AND entity_type IS NOT NULL
    )
  )
ORDER BY last_seen_at DESC, id DESC
LIMIT $2
OFFSET $3;

-- name: DismissRecommendation :one
UPDATE recommendations
SET
    status = 'dismissed',
    dismissed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: ResolveRecommendation :one
UPDATE recommendations
SET
    status = 'resolved',
    resolved_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: AcceptRecommendation :one
UPDATE recommendations
SET
    status = 'accepted',
    accepted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: CountOpenRecommendationsByPriority :many
SELECT
    priority_level,
    COUNT(*) AS recommendations_count
FROM recommendations
WHERE seller_account_id = $1
  AND status = 'open'
GROUP BY priority_level
ORDER BY priority_level ASC;

-- name: CountOpenRecommendationsByConfidence :many
SELECT
    confidence_level,
    COUNT(*) AS recommendations_count
FROM recommendations
WHERE seller_account_id = $1
  AND status = 'open'
GROUP BY confidence_level
ORDER BY confidence_level ASC;

-- name: CountOpenRecommendationsBySellerAccountID :one
SELECT COUNT(*)
FROM recommendations
WHERE seller_account_id = $1
  AND status = 'open';

-- name: CountRecommendationsBySellerAccountID :one
SELECT COUNT(*)::bigint
FROM recommendations
WHERE seller_account_id = $1;

-- name: CountRecommendationAlertLinksBySellerAccountID :one
SELECT COUNT(*)::bigint
FROM recommendation_alert_links
WHERE seller_account_id = $1;

-- name: CountRecommendationsWithNonEmptyPayloadsBySellerAccountID :one
SELECT COUNT(*)::bigint
FROM recommendations
WHERE seller_account_id = $1
  AND supporting_metrics_payload IS NOT NULL
  AND supporting_metrics_payload <> '{}'::jsonb
  AND constraints_payload IS NOT NULL
  AND constraints_payload <> '{}'::jsonb;

-- name: CountRecommendationsWithRawAIBySellerAccountID :one
SELECT COUNT(*)::bigint
FROM recommendations
WHERE seller_account_id = $1
  AND raw_ai_response IS NOT NULL
  AND raw_ai_response <> '{}'::jsonb;

-- name: CountRecommendationRunsBySellerAccountID :one
SELECT COUNT(*)::bigint
FROM recommendation_runs
WHERE seller_account_id = $1;

-- name: LinkRecommendationAlert :exec
INSERT INTO recommendation_alert_links (
    recommendation_id,
    alert_id,
    seller_account_id,
    link_type
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT (recommendation_id, alert_id) DO NOTHING;

-- name: ListAlertsByRecommendationID :many
SELECT a.*
FROM recommendation_alert_links ral
JOIN alerts a ON a.id = ral.alert_id
WHERE ral.seller_account_id = $1
  AND ral.recommendation_id = $2
ORDER BY a.last_seen_at DESC, a.id DESC;

-- name: ListRecommendationsByAlertID :many
SELECT r.*
FROM recommendation_alert_links ral
JOIN recommendations r ON r.id = ral.recommendation_id
WHERE ral.seller_account_id = $1
  AND ral.alert_id = $2
ORDER BY r.last_seen_at DESC, r.id DESC;

-- name: DeleteRecommendationAlertLinks :exec
DELETE FROM recommendation_alert_links
WHERE recommendation_id = $1
  AND seller_account_id = $2;

-- name: CreateRecommendationRun :one
INSERT INTO recommendation_runs (
    seller_account_id,
    run_type,
    as_of_date,
    ai_model,
    ai_prompt_version,
    status,
    started_at
) VALUES (
    $1, $2, $3, $4, $5, 'running', NOW()
)
RETURNING *;

-- name: CompleteRecommendationRun :one
UPDATE recommendation_runs
SET
    status = 'completed',
    finished_at = NOW(),
    input_tokens = $3,
    output_tokens = $4,
    estimated_cost = $5,
    generated_recommendations_count = $6,
    accepted_recommendations_count = $7,
    error_message = NULL
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: FailRecommendationRun :one
UPDATE recommendation_runs
SET
    status = 'failed',
    finished_at = NOW(),
    error_message = $3
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: GetLatestRecommendationRunBySellerAccountID :one
SELECT *
FROM recommendation_runs
WHERE seller_account_id = $1
ORDER BY started_at DESC, id DESC
LIMIT 1;

-- name: ListRecommendationRunsBySellerAccountID :many
SELECT *
FROM recommendation_runs
WHERE seller_account_id = $1
ORDER BY started_at DESC, id DESC
LIMIT $2
OFFSET $3;
