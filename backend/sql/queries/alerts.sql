-- name: UpsertAlert :one
INSERT INTO alerts (
    seller_account_id,
    alert_type,
    alert_group,
    entity_type,
    entity_id,
    entity_sku,
    entity_offer_id,
    title,
    message,
    severity,
    urgency,
    status,
    evidence_payload,
    fingerprint,
    first_seen_at,
    last_seen_at,
    resolved_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'open', $12, $13, NOW(), NOW(), NULL, NOW()
)
ON CONFLICT (seller_account_id, fingerprint)
DO UPDATE SET
    alert_type = CASE
        WHEN alerts.status = 'dismissed' THEN alerts.alert_type
        ELSE EXCLUDED.alert_type
    END,
    alert_group = CASE
        WHEN alerts.status = 'dismissed' THEN alerts.alert_group
        ELSE EXCLUDED.alert_group
    END,
    entity_type = CASE
        WHEN alerts.status = 'dismissed' THEN alerts.entity_type
        ELSE EXCLUDED.entity_type
    END,
    entity_id = CASE
        WHEN alerts.status = 'dismissed' THEN alerts.entity_id
        ELSE EXCLUDED.entity_id
    END,
    entity_sku = CASE
        WHEN alerts.status = 'dismissed' THEN alerts.entity_sku
        ELSE EXCLUDED.entity_sku
    END,
    entity_offer_id = CASE
        WHEN alerts.status = 'dismissed' THEN alerts.entity_offer_id
        ELSE EXCLUDED.entity_offer_id
    END,
    title = CASE
        WHEN alerts.status = 'dismissed' THEN alerts.title
        ELSE EXCLUDED.title
    END,
    message = CASE
        WHEN alerts.status = 'dismissed' THEN alerts.message
        ELSE EXCLUDED.message
    END,
    severity = CASE
        WHEN alerts.status = 'dismissed' THEN alerts.severity
        ELSE EXCLUDED.severity
    END,
    urgency = CASE
        WHEN alerts.status = 'dismissed' THEN alerts.urgency
        ELSE EXCLUDED.urgency
    END,
    status = CASE
        WHEN alerts.status = 'dismissed' THEN alerts.status
        ELSE 'open'
    END,
    evidence_payload = EXCLUDED.evidence_payload,
    last_seen_at = NOW(),
    resolved_at = CASE
        WHEN alerts.status = 'dismissed' THEN alerts.resolved_at
        ELSE NULL
    END,
    updated_at = NOW()
RETURNING *;

-- name: GetAlertByID :one
SELECT *
FROM alerts
WHERE id = $1
  AND seller_account_id = $2;

-- name: ListOpenAlertsBySellerAccountID :many
SELECT *
FROM alerts
WHERE seller_account_id = $1
  AND status = 'open'
ORDER BY last_seen_at DESC, id DESC
LIMIT $2
OFFSET $3;

-- name: ListAlertsBySellerAccountID :many
SELECT *
FROM alerts
WHERE seller_account_id = $1
ORDER BY last_seen_at DESC, id DESC
LIMIT $2
OFFSET $3;

-- name: ListAlertsFiltered :many
SELECT *
FROM alerts
WHERE seller_account_id = $1
  AND (
    status = sqlc.narg(status)
    OR (
      sqlc.narg(status)::text IS NULL
      AND status IS NOT NULL
    )
  )
  AND (
    alert_group = sqlc.narg(alert_group)
    OR (
      sqlc.narg(alert_group)::text IS NULL
      AND alert_group IS NOT NULL
    )
  )
  AND (
    severity = sqlc.narg(severity)
    OR (
      sqlc.narg(severity)::text IS NULL
      AND severity IS NOT NULL
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

-- name: ListAlertsByGroup :many
SELECT *
FROM alerts
WHERE seller_account_id = $1
  AND alert_group = $2
ORDER BY last_seen_at DESC, id DESC
LIMIT $3
OFFSET $4;

-- name: ListAlertsByEntitySKU :many
SELECT *
FROM alerts
WHERE seller_account_id = $1
  AND entity_sku = $2
ORDER BY last_seen_at DESC, id DESC
LIMIT $3
OFFSET $4;

-- name: ListAlertsByEntityID :many
SELECT *
FROM alerts
WHERE seller_account_id = $1
  AND entity_type = $2
  AND entity_id = $3
ORDER BY last_seen_at DESC, id DESC
LIMIT $4
OFFSET $5;

-- name: ResolveAlert :one
UPDATE alerts
SET
    status = 'resolved',
    resolved_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: DismissAlert :one
UPDATE alerts
SET
    status = 'dismissed',
    resolved_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: CountOpenAlertsBySeverity :many
SELECT
    severity,
    COUNT(*) AS alerts_count
FROM alerts
WHERE seller_account_id = $1
  AND status = 'open'
GROUP BY severity
ORDER BY severity ASC;

-- name: CountOpenAlertsByGroup :many
SELECT
    alert_group,
    COUNT(*) AS alerts_count
FROM alerts
WHERE seller_account_id = $1
  AND status = 'open'
GROUP BY alert_group
ORDER BY alert_group ASC;

-- name: CountOpenAlertsBySellerAccountID :one
SELECT COUNT(*)
FROM alerts
WHERE seller_account_id = $1
  AND status = 'open';

-- name: CountAlertsBySellerAccountID :one
SELECT COUNT(*)::bigint
FROM alerts
WHERE seller_account_id = $1;

-- name: DeleteAlertsBySellerAccountID :exec
DELETE FROM alerts
WHERE seller_account_id = $1;

-- name: DeleteAlertRunsBySellerAccountID :exec
DELETE FROM alert_runs
WHERE seller_account_id = $1;

-- name: CountAlertRunsBySellerAccountID :one
SELECT COUNT(*)::bigint
FROM alert_runs
WHERE seller_account_id = $1;

-- name: CreateAlertRun :one
INSERT INTO alert_runs (
    seller_account_id,
    run_type,
    status
) VALUES (
    $1, $2, 'running'
)
RETURNING *;

-- name: CompleteAlertRun :one
UPDATE alert_runs
SET
    status = 'completed',
    finished_at = NOW(),
    sales_alerts_count = $3,
    stock_alerts_count = $4,
    ad_alerts_count = $5,
    price_alerts_count = $6,
    total_alerts_count = $7,
    error_message = NULL
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: FailAlertRun :one
UPDATE alert_runs
SET
    status = 'failed',
    finished_at = NOW(),
    error_message = $3
WHERE id = $1
  AND seller_account_id = $2
RETURNING *;

-- name: GetLatestAlertRunBySellerAccountID :one
SELECT *
FROM alert_runs
WHERE seller_account_id = $1
ORDER BY started_at DESC, id DESC
LIMIT 1;

-- name: ListAlertRunsBySellerAccountID :many
SELECT *
FROM alert_runs
WHERE seller_account_id = $1
ORDER BY started_at DESC, id DESC
LIMIT $2
OFFSET $3;
