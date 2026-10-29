-- Session ↔ action_key (one chat per action per seller)
ALTER TABLE chat_sessions
    ADD COLUMN IF NOT EXISTS action_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS chat_sessions_seller_action_key_uniq
    ON chat_sessions (seller_account_id, action_key)
    WHERE action_key IS NOT NULL AND TRIM(action_key) <> '';

-- Idempotent ask via client_request_id
ALTER TABLE chat_messages
    ADD COLUMN IF NOT EXISTS client_request_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS chat_messages_client_request_id_uniq
    ON chat_messages (seller_account_id, client_request_id)
    WHERE client_request_id IS NOT NULL AND TRIM(client_request_id) <> '';

-- Input attachments (draft uploads before message send)
ALTER TABLE chat_attachments
    ALTER COLUMN chat_session_id DROP NOT NULL,
    ALTER COLUMN chat_message_id DROP NOT NULL,
    ALTER COLUMN export_type DROP NOT NULL;

ALTER TABLE chat_attachments
    ADD COLUMN IF NOT EXISTS attachment_kind TEXT NOT NULL DEFAULT 'export',
    ADD COLUMN IF NOT EXISTS openai_file_id TEXT,
    ADD COLUMN IF NOT EXISTS openai_uploaded_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS sha256 TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE chat_attachments SET attachment_kind = 'export' WHERE attachment_kind IS NULL OR attachment_kind = '';

CREATE INDEX IF NOT EXISTS chat_attachments_draft_expires_idx
    ON chat_attachments (seller_account_id, status, expires_at)
    WHERE chat_message_id IS NULL;
