ALTER TABLE ozon_connections
    ADD COLUMN performance_token_encrypted TEXT NULL,
    ADD COLUMN performance_status TEXT NOT NULL DEFAULT 'not_configured',
    ADD COLUMN performance_last_check_at TIMESTAMPTZ NULL,
    ADD COLUMN performance_last_check_result TEXT NULL,
    ADD COLUMN performance_last_error TEXT NULL,
    ADD CONSTRAINT ozon_connections_performance_status_check CHECK (
        performance_status IN (
            'not_configured',
            'unknown',
            'valid',
            'invalid'
        )
    );

COMMENT ON COLUMN ozon_connections.performance_token_encrypted IS 'Encrypted Ozon Performance API bearer token (separate from Seller API key).';
