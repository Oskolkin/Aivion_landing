-- name: CreateRawPayload :one
INSERT INTO raw_payloads (
    seller_account_id,
    import_job_id,
    domain,
    source,
    request_key,
    storage_bucket,
    storage_object_key,
    payload_hash,
    received_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, NOW()
)
RETURNING *;

-- name: ListRawPayloadsByImportJobID :many
SELECT *
FROM raw_payloads
WHERE import_job_id = $1
ORDER BY received_at DESC;