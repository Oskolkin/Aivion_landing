CREATE TABLE chat_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    chat_session_id BIGINT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    chat_message_id BIGINT NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    export_type TEXT NOT NULL,
    format TEXT NOT NULL,
    file_name TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    file_size BIGINT NOT NULL DEFAULT 0,
    storage_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ready',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);

CREATE INDEX chat_attachments_seller_session_idx
    ON chat_attachments (seller_account_id, chat_session_id);

CREATE INDEX chat_attachments_message_idx ON chat_attachments (chat_message_id);

CREATE INDEX chat_attachments_user_idx ON chat_attachments (created_by_user_id);

CREATE INDEX chat_attachments_created_at_idx ON chat_attachments (created_at DESC);
