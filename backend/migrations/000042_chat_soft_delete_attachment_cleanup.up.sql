-- Soft-delete chat sessions (allows new action chat after delete without unique conflict)
ALTER TABLE chat_sessions
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

DROP INDEX IF EXISTS chat_sessions_seller_action_key_uniq;
CREATE UNIQUE INDEX chat_sessions_seller_action_key_uniq
    ON chat_sessions (seller_account_id, action_key)
    WHERE action_key IS NOT NULL
      AND TRIM(action_key) <> ''
      AND deleted_at IS NULL;

-- Attachment OpenAI/MinIO cleanup tracking
ALTER TABLE chat_attachments
    ADD COLUMN IF NOT EXISTS openai_deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS openai_cleanup_pending BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS cleaned_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cleanup_retry_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cleanup_error_code TEXT,
    ADD COLUMN IF NOT EXISTS warning_code TEXT;

CREATE INDEX IF NOT EXISTS chat_attachments_openai_cleanup_pending_idx
    ON chat_attachments (seller_account_id, openai_cleanup_pending)
    WHERE openai_cleanup_pending = TRUE
      AND openai_file_id IS NOT NULL
      AND TRIM(openai_file_id) <> '';

CREATE INDEX IF NOT EXISTS chat_attachments_expired_draft_idx
    ON chat_attachments (status, expires_at)
    WHERE chat_message_id IS NULL
      AND attachment_kind = 'input';
