-- name: UpsertSellerAnalysisSettings :one
INSERT INTO seller_analysis_settings (
    seller_account_id,
    analysis_mode,
    auto_run_enabled,
    auto_publish_enabled,
    legacy_fallback_enabled,
    updated_at,
    updated_by
) VALUES (
    $1, $2, $3, $4, $5, now(), $6
)
ON CONFLICT (seller_account_id) DO UPDATE
SET
    analysis_mode = EXCLUDED.analysis_mode,
    auto_run_enabled = EXCLUDED.auto_run_enabled,
    auto_publish_enabled = EXCLUDED.auto_publish_enabled,
    legacy_fallback_enabled = EXCLUDED.legacy_fallback_enabled,
    updated_at = now(),
    updated_by = EXCLUDED.updated_by
RETURNING *;

-- name: GetSellerAnalysisSettings :one
SELECT *
FROM seller_analysis_settings
WHERE seller_account_id = $1;

-- name: CreateUnifiedAnalysisPublication :one
INSERT INTO unified_analysis_publications (
    seller_account_id,
    metric_date,
    run_id,
    pack_id,
    pack_hash,
    prompt_version,
    prompt_hash,
    schema_version,
    schema_hash,
    output_hash,
    adapter_version,
    adapter_hash,
    mode,
    visibility,
    status,
    analysis_outcome,
    dashboard_run_id,
    actions_run_id,
    dashboard_data_snapshot_id,
    dashboard_record_count,
    actions_record_count,
    top_actions_count,
    recommended_actions_count,
    dashboard_projection_hash,
    actions_projection_hash,
    bundle_hash
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19,
    $20, $21, $22, $23, $24, $25, $26
)
RETURNING *;

-- name: GetUnifiedAnalysisPublicationByID :one
SELECT *
FROM unified_analysis_publications
WHERE id = $1;

-- name: GetUnifiedAnalysisPublicationByRunID :one
SELECT *
FROM unified_analysis_publications
WHERE run_id = $1;

-- name: GetUnifiedAnalysisPublicationByIDForUpdate :one
SELECT *
FROM unified_analysis_publications
WHERE id = $1
FOR UPDATE;

-- name: ListUnifiedAnalysisPublicationsBySeller :many
SELECT *
FROM unified_analysis_publications
WHERE seller_account_id = $1
ORDER BY metric_date DESC, id DESC
LIMIT $2;

-- name: UpdateUnifiedAnalysisPublicationStatus :one
UPDATE unified_analysis_publications
SET
    status = $2,
    error_code = $3,
    error_message = $4,
    published_at = CASE WHEN $2 = 'published' THEN COALESCE(published_at, now()) ELSE published_at END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkUnifiedAnalysisPublicationPublished :one
UPDATE unified_analysis_publications
SET
    status = 'published',
    visibility = $2,
    dashboard_run_id = $3,
    actions_run_id = $4,
    dashboard_data_snapshot_id = $5,
    dashboard_record_count = $6,
    actions_record_count = $7,
    top_actions_count = $8,
    recommended_actions_count = $9,
    dashboard_projection_hash = $10,
    actions_projection_hash = $11,
    bundle_hash = $12,
    published_at = COALESCE(published_at, now()),
    error_code = NULL,
    error_message = NULL,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkUnifiedAnalysisPublicationSuperseded :one
UPDATE unified_analysis_publications
SET
    status = 'superseded',
    superseded_at = now(),
    superseded_by_publication_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpsertSellerCurrentUnifiedPublication :one
INSERT INTO seller_current_unified_publications (
    seller_account_id,
    publication_id,
    metric_date,
    source_mode,
    updated_at
) VALUES (
    $1, $2, $3, 'unified', now()
)
ON CONFLICT (seller_account_id) DO UPDATE
SET
    publication_id = EXCLUDED.publication_id,
    metric_date = EXCLUDED.metric_date,
    source_mode = 'unified',
    updated_at = now()
RETURNING *;

-- name: GetSellerCurrentUnifiedPublication :one
SELECT
    c.seller_account_id,
    c.publication_id,
    c.metric_date,
    c.source_mode,
    c.updated_at,
    p.run_id,
    p.visibility,
    p.status,
    p.analysis_outcome,
    p.output_hash,
    p.bundle_hash,
    p.actions_run_id,
    p.dashboard_run_id,
    p.dashboard_data_snapshot_id
FROM seller_current_unified_publications c
JOIN unified_analysis_publications p ON p.id = c.publication_id
WHERE c.seller_account_id = $1;

-- name: GetSellerCurrentUnifiedPublicationForUpdate :one
SELECT
    c.seller_account_id,
    c.publication_id,
    c.metric_date,
    c.source_mode,
    c.updated_at
FROM seller_current_unified_publications c
WHERE c.seller_account_id = $1
FOR UPDATE;

-- name: DeleteSellerCurrentUnifiedPublication :exec
DELETE FROM seller_current_unified_publications
WHERE seller_account_id = $1;

-- name: InsertUnifiedPublicationActionMap :one
INSERT INTO unified_publication_action_map (
    publication_id,
    analysis_action_id,
    action_item_id,
    is_top,
    is_recommended,
    sort_order
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ListUnifiedPublicationActionMaps :many
SELECT *
FROM unified_publication_action_map
WHERE publication_id = $1
ORDER BY sort_order ASC, id ASC;

-- name: InsertUnifiedAnalysisPublicationAudit :one
INSERT INTO unified_analysis_publication_audit (
    seller_account_id,
    operation,
    actor,
    run_id,
    publication_id,
    previous_publication_id,
    new_publication_id,
    reason,
    payload_json
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: SetAIPipelineUnifiedPublicationID :one
UPDATE ai_pipeline_runs
SET
    unified_publication_id = $2,
    updated_at = now()
WHERE id = $1
  AND mode = 'unified_analysis'
RETURNING *;
