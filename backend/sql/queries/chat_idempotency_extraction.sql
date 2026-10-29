-- name: UpsertChatRequestIdempotency :one
INSERT INTO chat_request_idempotency (
    seller_account_id,
    user_id,
    client_request_id,
    action_id,
    action_key,
    publication_id,
    status,
    request_hash,
    request_type,
    started_at,
    heartbeat_at,
    lease_until,
    owner_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW(), $10, $11
)
ON CONFLICT (seller_account_id, client_request_id)
DO UPDATE SET updated_at = NOW()
RETURNING *;

-- name: GetChatRequestIdempotency :one
SELECT * FROM chat_request_idempotency
WHERE seller_account_id = $1 AND client_request_id = $2;

-- name: UpdateChatRequestIdempotency :one
UPDATE chat_request_idempotency
SET
    status = $3,
    session_id = COALESCE($4, session_id),
    user_message_id = COALESCE($5, user_message_id),
    assistant_message_id = COALESCE($6, assistant_message_id),
    response_payload = COALESCE($7, response_payload),
    error_code = $8,
    error_message = $9,
    lease_until = CASE WHEN $3 IN ('completed', 'failed', 'cancelled') THEN NULL ELSE lease_until END,
    owner_id = CASE WHEN $3 IN ('completed', 'failed', 'cancelled') THEN NULL ELSE owner_id END,
    updated_at = NOW()
WHERE seller_account_id = $1 AND client_request_id = $2 AND owner_id = $10
RETURNING *;

-- name: ReclaimChatRequestIdempotency :one
UPDATE chat_request_idempotency
SET
    status = 'running',
    started_at = NOW(),
    heartbeat_at = NOW(),
    lease_until = $4,
    owner_id = $5,
    updated_at = NOW()
WHERE seller_account_id = $1
  AND client_request_id = $2
  AND request_hash = $3
  AND (status <> 'running' OR lease_until IS NULL OR lease_until <= NOW())
RETURNING *;

-- name: HeartbeatChatRequestIdempotency :one
UPDATE chat_request_idempotency
SET heartbeat_at = NOW(), lease_until = $4, updated_at = NOW()
WHERE seller_account_id = $1 AND client_request_id = $2 AND owner_id = $3 AND status = 'running'
RETURNING *;

-- name: SetChatAttachmentExtraction :one
UPDATE chat_attachments
SET
    extraction_status = $3,
    extracted_text = $4,
    extracted_meta = $5,
    page_count = $6,
    sheet_count = $7,
    slide_count = $8,
    character_count = $9,
    extraction_error = $10,
    extracted_at = CASE WHEN $3 = 'ready' THEN NOW() ELSE extracted_at END,
    updated_at = NOW()
WHERE seller_account_id = $1 AND id = $2
RETURNING *;
