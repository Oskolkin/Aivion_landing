DROP INDEX IF EXISTS chat_attachments_expired_draft_idx;
DROP INDEX IF EXISTS chat_attachments_openai_cleanup_pending_idx;

ALTER TABLE chat_attachments
    DROP COLUMN IF EXISTS warning_code,
    DROP COLUMN IF EXISTS cleanup_error_code,
    DROP COLUMN IF EXISTS cleanup_retry_count,
    DROP COLUMN IF EXISTS cleaned_at,
    DROP COLUMN IF EXISTS openai_cleanup_pending,
    DROP COLUMN IF EXISTS openai_deleted_at;

DROP INDEX IF EXISTS chat_sessions_seller_action_key_uniq;
CREATE UNIQUE INDEX chat_sessions_seller_action_key_uniq
    ON chat_sessions (seller_account_id, action_key)
    WHERE action_key IS NOT NULL AND TRIM(action_key) <> '';

ALTER TABLE chat_sessions
    DROP COLUMN IF EXISTS deleted_at;
