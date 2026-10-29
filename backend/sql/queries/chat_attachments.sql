-- name: CreateChatAttachment :one
INSERT INTO chat_attachments (
    id,
    seller_account_id,
    chat_session_id,
    chat_message_id,
    created_by_user_id,
    export_type,
    format,
    file_name,
    mime_type,
    file_size,
    storage_key,
    status,
    error_message,
    expires_at,
    attachment_kind,
    sha256
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
)
RETURNING *;

-- name: GetChatAttachmentByID :one
SELECT *
FROM chat_attachments
WHERE id = $1;

-- name: GetChatAttachmentByIDForSeller :one
SELECT *
FROM chat_attachments
WHERE id = $1
  AND seller_account_id = $2;

-- name: ListChatAttachmentsByMessageIDs :many
SELECT *
FROM chat_attachments
WHERE seller_account_id = $1
  AND chat_message_id = ANY($2::bigint[])
ORDER BY created_at ASC;

-- name: UpdateChatAttachmentStatus :one
UPDATE chat_attachments
SET
    status = $3,
    error_message = $4,
    file_size = COALESCE($5, file_size),
    updated_at = NOW()
WHERE seller_account_id = $1
  AND id = $2
RETURNING *;

-- name: AttachChatAttachmentToMessage :one
UPDATE chat_attachments
SET
    chat_session_id = $3,
    chat_message_id = $4,
    status = $5,
    openai_file_id = $6,
    openai_uploaded_at = $7,
    updated_at = NOW()
WHERE seller_account_id = $1
  AND id = $2
RETURNING *;

-- name: DeleteDraftChatAttachment :exec
DELETE FROM chat_attachments
WHERE seller_account_id = $1
  AND id = $2
  AND chat_message_id IS NULL
  AND attachment_kind = 'input';

-- name: CountDraftInputChatAttachments :one
SELECT COUNT(*)::bigint
FROM chat_attachments
WHERE seller_account_id = $1
  AND attachment_kind = 'input'
  AND chat_message_id IS NULL
  AND cleaned_at IS NULL
  AND status NOT IN ('deleted');

-- name: ListDraftChatAttachmentsByIDs :many
SELECT *
FROM chat_attachments
WHERE seller_account_id = $1
  AND id = ANY($2::uuid[])
  AND attachment_kind = 'input'
  AND chat_message_id IS NULL;

-- name: ListExpiredDraftChatAttachments :many
SELECT *
FROM chat_attachments
WHERE chat_message_id IS NULL
  AND attachment_kind = 'input'
  AND status IN ('uploaded', 'failed')
  AND expires_at IS NOT NULL
  AND expires_at < NOW()
  AND cleaned_at IS NULL
ORDER BY expires_at ASC
LIMIT $1;

-- name: ListOpenAICleanupPendingAttachments :many
SELECT *
FROM chat_attachments
WHERE openai_cleanup_pending = TRUE
  AND openai_file_id IS NOT NULL
  AND TRIM(openai_file_id) <> ''
  AND openai_deleted_at IS NULL
  AND cleanup_retry_count < $1
ORDER BY updated_at ASC
LIMIT $2;

-- name: MarkChatAttachmentCleaned :one
UPDATE chat_attachments
SET
    status = $3,
    cleaned_at = NOW(),
    updated_at = NOW(),
    cleanup_error_code = $4
WHERE seller_account_id = $1
  AND id = $2
RETURNING *;

-- name: MarkChatAttachmentOpenAIDeleted :one
UPDATE chat_attachments
SET
    openai_deleted_at = NOW(),
    openai_cleanup_pending = FALSE,
    openai_file_id = NULL,
    updated_at = NOW()
WHERE seller_account_id = $1
  AND id = $2
RETURNING *;

-- name: MarkChatAttachmentOpenAICleanupPending :exec
UPDATE chat_attachments
SET
    openai_cleanup_pending = TRUE,
    updated_at = NOW()
WHERE seller_account_id = $1
  AND id = $2;

-- name: IncrementChatAttachmentCleanupRetry :exec
UPDATE chat_attachments
SET
    cleanup_retry_count = cleanup_retry_count + 1,
    cleanup_error_code = $3,
    updated_at = NOW()
WHERE seller_account_id = $1
  AND id = $2;

-- name: SetChatAttachmentWarningCode :exec
UPDATE chat_attachments
SET
    warning_code = $3,
    updated_at = NOW()
WHERE seller_account_id = $1
  AND id = $2;
