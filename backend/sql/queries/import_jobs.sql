-- name: CreateImportJob :one
INSERT INTO import_jobs (
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
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW()
)
RETURNING *;

-- name: EnsureImportJob :one
INSERT INTO import_jobs (
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
) VALUES (
    $1, $2, $3, 'pending', $4, 0, 0, 0, NULL, NULL, NULL, NOW()
)
ON CONFLICT (sync_job_id, domain) WHERE superseded_by_import_job_id IS NULL AND status <> 'superseded' DO UPDATE
SET seller_account_id = import_jobs.seller_account_id
RETURNING *;

-- name: GetImportJobBySyncJobAndDomain :one
SELECT *
FROM import_jobs
WHERE sync_job_id = $1
  AND domain = $2
LIMIT 1;

-- name: UpdateImportJobToFetching :one
UPDATE import_jobs
SET
    status = 'fetching',
    started_at = COALESCE(started_at, NOW()),
    error_message = NULL
WHERE id = $1
  AND status IN ('pending', 'retrying')
RETURNING *;

-- name: UpdateImportJobToImporting :one
UPDATE import_jobs
SET
    status = 'importing'
WHERE id = $1
  AND status IN ('fetching', 'retrying')
RETURNING *;

-- name: UpdateImportJobToRetrying :one
UPDATE import_jobs
SET
    status = 'retrying',
    error_message = $2
WHERE id = $1
  AND status IN ('pending', 'fetching', 'importing', 'retrying')
RETURNING *;

-- name: UpdateImportJobToCompleted :one
UPDATE import_jobs
SET
    status = 'completed',
    finished_at = NOW(),
    records_received = $2,
    records_imported = $3,
    records_failed = $4,
    error_message = NULL
WHERE id = $1
  AND status IN ('fetching', 'importing', 'retrying')
RETURNING *;

-- name: UpdateImportJobToFailed :one
UPDATE import_jobs
SET
    status = 'failed',
    finished_at = NOW(),
    error_message = $2
WHERE id = $1
  AND status NOT IN ('completed', 'failed')
RETURNING *;

-- name: FailPendingImportJobsForSync :exec
UPDATE import_jobs
SET
    status = 'failed',
    finished_at = NOW(),
    error_message = $2
WHERE sync_job_id = $1
  AND status IN ('pending', 'fetching', 'importing', 'retrying');

-- name: ListImportJobsBySyncJobID :many
SELECT *
FROM import_jobs
WHERE sync_job_id = $1
ORDER BY id ASC;

-- name: ListCanonicalImportJobsBySyncJobID :many
SELECT *
FROM import_jobs
WHERE sync_job_id = $1
  AND superseded_by_import_job_id IS NULL
  AND status <> 'superseded'
  AND domain !~ '__dedup_[0-9]+$'
ORDER BY id ASC;

-- name: ListLatestImportJobsBySellerAccountID :many
SELECT DISTINCT ON (domain) *
FROM import_jobs
WHERE seller_account_id = $1
ORDER BY domain, created_at DESC, id DESC;

-- name: GetImportJobByID :one
SELECT *
FROM import_jobs
WHERE id = $1
LIMIT 1;

-- name: CountImportJobsSummaryBySyncJobID :one
SELECT
    COUNT(*)::bigint AS total_count,
    COUNT(*) FILTER (WHERE status IN ('pending', 'fetching', 'importing', 'retrying'))::bigint AS active_count,
    COUNT(*) FILTER (WHERE status = 'failed')::bigint AS failed_count,
    COUNT(*) FILTER (WHERE status = 'completed')::bigint AS completed_count
FROM import_jobs
WHERE sync_job_id = $1
  AND superseded_by_import_job_id IS NULL
  AND status <> 'superseded'
  AND domain !~ '__dedup_[0-9]+$';
