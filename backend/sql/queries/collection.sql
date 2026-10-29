-- name: CreateCollectionRun :one
INSERT INTO collection_runs (seller_account_id, kind, status, trigger, trigger_metadata, requested_by)
VALUES ($1, $2, 'PENDING', $3, $4::jsonb, $5)
RETURNING *;

-- name: GetCollectionRunForSeller :one
SELECT * FROM collection_runs WHERE id = $1 AND seller_account_id = $2;

-- name: TransitionCollectionRunForSeller :one
UPDATE collection_runs
SET status = sqlc.arg(status), started_at = CASE WHEN sqlc.arg(status) = 'RUNNING' THEN NOW() ELSE started_at END,
    finished_at = CASE WHEN sqlc.arg(status) IN ('COMPLETED', 'COMPLETED_WITH_LIMITATIONS', 'FAILED', 'CANCELLED') THEN NOW() ELSE finished_at END,
    updated_at = NOW()
WHERE id = sqlc.arg(id) AND seller_account_id = sqlc.arg(seller_account_id)
  AND status = sqlc.arg(expected_status)
RETURNING *;

-- name: CreateCollectionDomainRun :one
INSERT INTO collection_domain_runs (
  collection_run_id, seller_account_id, domain, capability_key, requested_from, requested_to,
  source_contract_key, source_contract_version, source_endpoint_family,
  semantic_mapping_version, semantic_mapping_hash, source_revision_hash
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetCollectionDomainRunForSeller :one
SELECT * FROM collection_domain_runs WHERE id = $1 AND seller_account_id = $2;

-- name: ListCollectionDomainRunsForSeller :many
SELECT * FROM collection_domain_runs WHERE collection_run_id = $1 AND seller_account_id = $2 ORDER BY id;

-- name: TransitionCollectionDomainRunForSeller :one
UPDATE collection_domain_runs
SET status = sqlc.arg(status), started_at = CASE WHEN sqlc.arg(status) = 'RUNNING' THEN NOW() ELSE started_at END,
    finished_at = CASE WHEN sqlc.arg(status) IN ('COMPLETED', 'PARTIAL', 'FAILED', 'CANCELLED', 'UNAVAILABLE', 'NOT_APPLICABLE') THEN NOW() ELSE finished_at END,
    updated_at = NOW()
WHERE id = sqlc.arg(id) AND seller_account_id = sqlc.arg(seller_account_id)
  AND status = sqlc.arg(expected_status)
RETURNING *;

-- name: GetCollectionCursorForSeller :one
SELECT * FROM collection_cursors WHERE seller_account_id = $1 AND domain = $2 AND cursor_key = $3;

-- name: UpsertCollectionCursor :one
INSERT INTO collection_cursors (seller_account_id, domain, cursor_key, cursor_value)
VALUES ($1, $2, $3, $4::jsonb)
ON CONFLICT (seller_account_id, domain, cursor_key) DO UPDATE
SET cursor_value = EXCLUDED.cursor_value, updated_at = NOW()
RETURNING *;

-- name: CreateSourcePayload :one
INSERT INTO source_payloads (seller_account_id, collection_domain_run_id, provider, endpoint_key, request_fingerprint, page_seq, storage_bucket, storage_object_key, sha256, http_status, received_at, metadata_json, redaction_profile)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13)
RETURNING *;

-- name: GetSourcePayloadForSeller :one
SELECT * FROM source_payloads WHERE id = $1 AND seller_account_id = $2;

-- name: ListSourcePayloadsForDomainRunForSeller :many
SELECT * FROM source_payloads WHERE collection_domain_run_id = $1 AND seller_account_id = $2 ORDER BY received_at, id;

-- name: UpdateCollectionDomainRunProgress :one
UPDATE collection_domain_runs SET records_received=$3,records_written=$4,pages_received=$5,completeness_status=$6,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2 RETURNING *;

-- name: CancelSupersededCollectionDomainRun :one
UPDATE collection_domain_runs
SET status='CANCELLED', finished_at=NOW(), completeness_status='UNKNOWN',
    error_code='SUPERSEDED_BY_NEWER_AUTHORITATIVE_RUN', error_message=$3, updated_at=NOW()
WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'
RETURNING *;

-- name: FailCollectionDomainRunWithCode :one
UPDATE collection_domain_runs
SET status='FAILED', finished_at=NOW(), completeness_status='UNKNOWN',
    error_code=$3, error_message=$4, updated_at=NOW()
WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'
RETURNING *;
