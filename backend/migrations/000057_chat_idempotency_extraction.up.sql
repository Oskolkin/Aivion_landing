-- Server-side chat request idempotency + attachment extraction audit columns.

CREATE TABLE IF NOT EXISTS chat_request_idempotency (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    client_request_id TEXT NOT NULL,
    action_id BIGINT,
    action_key TEXT,
    publication_id BIGINT,
    session_id BIGINT REFERENCES chat_sessions(id) ON DELETE SET NULL,
    user_message_id BIGINT REFERENCES chat_messages(id) ON DELETE SET NULL,
    assistant_message_id BIGINT REFERENCES chat_messages(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    response_payload JSONB,
    error_code TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chat_request_idempotency_status_chk CHECK (
        status IN (
            'queued',
            'creating_or_loading_conversation',
            'attaching_context',
            'message_created',
            'running',
            'streaming',
            'completed',
            'failed',
            'cancelled'
        )
    ),
    CONSTRAINT chat_request_idempotency_seller_client_uniq UNIQUE (seller_account_id, client_request_id)
);

CREATE INDEX chat_request_idempotency_seller_updated_idx
    ON chat_request_idempotency (seller_account_id, updated_at DESC);

CREATE INDEX chat_request_idempotency_action_idx
    ON chat_request_idempotency (seller_account_id, action_id)
    WHERE action_id IS NOT NULL;

ALTER TABLE chat_attachments
    ADD COLUMN IF NOT EXISTS extraction_status TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS extracted_text TEXT,
    ADD COLUMN IF NOT EXISTS extracted_meta JSONB,
    ADD COLUMN IF NOT EXISTS page_count INT,
    ADD COLUMN IF NOT EXISTS sheet_count INT,
    ADD COLUMN IF NOT EXISTS slide_count INT,
    ADD COLUMN IF NOT EXISTS character_count INT,
    ADD COLUMN IF NOT EXISTS extraction_error TEXT,
    ADD COLUMN IF NOT EXISTS extracted_at TIMESTAMPTZ;

ALTER TABLE chat_attachments
    DROP CONSTRAINT IF EXISTS chat_attachments_extraction_status_chk;

ALTER TABLE chat_attachments
    ADD CONSTRAINT chat_attachments_extraction_status_chk CHECK (
        extraction_status IN (
            'pending',
            'extracting',
            'ready',
            'failed',
            'unsupported',
            'skipped'
        )
    );
