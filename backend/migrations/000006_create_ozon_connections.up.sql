CREATE TABLE ozon_connections (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL UNIQUE REFERENCES seller_accounts(id) ON DELETE CASCADE,
    client_id_encrypted TEXT NOT NULL,
    api_key_encrypted TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    last_check_at TIMESTAMPTZ NULL,
    last_check_result TEXT NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ozon_connections_status_check CHECK (
        status IN (
            'draft',
            'checking',
            'valid',
            'invalid',
            'sync_pending',
            'sync_in_progress',
            'sync_failed'
        )
    )
);

CREATE INDEX idx_ozon_connections_seller_account_id ON ozon_connections(seller_account_id);
CREATE INDEX idx_ozon_connections_status ON ozon_connections(status);