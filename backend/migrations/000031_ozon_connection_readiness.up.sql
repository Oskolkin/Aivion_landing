ALTER TABLE ozon_connections
    ADD COLUMN performance_client_id_encrypted TEXT NULL,
    ADD COLUMN performance_client_secret_encrypted TEXT NULL,
    ADD COLUMN performance_token_expires_at TIMESTAMPTZ NULL,
    ADD COLUMN seller_api_valid BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN performance_api_valid BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN connection_status TEXT NOT NULL DEFAULT 'not_connected',
    ADD COLUMN connection_error_code TEXT NULL,
    ADD COLUMN connection_error_message TEXT NULL,
    ADD COLUMN disconnected_at TIMESTAMPTZ NULL,
    ADD COLUMN retention_delete_after TIMESTAMPTZ NULL;

ALTER TABLE ozon_connections
    ADD CONSTRAINT ozon_connections_connection_status_check CHECK (
        connection_status IN (
            'not_connected',
            'connected',
            'connection_error',
            'disconnected_retention'
        )
    );

-- Backfill readiness from legacy status columns where possible.
UPDATE ozon_connections
SET
    seller_api_valid = (status = 'valid'),
    performance_api_valid = (performance_status = 'valid'),
    connection_status = CASE
        WHEN status = 'valid' AND performance_status = 'valid' THEN 'connected'
        WHEN status IN ('invalid') OR performance_status = 'invalid' THEN 'connection_error'
        ELSE 'not_connected'
    END
WHERE connection_status = 'not_connected';

COMMENT ON COLUMN ozon_connections.performance_client_id_encrypted IS 'Encrypted Ozon Performance API client_id (OAuth).';
COMMENT ON COLUMN ozon_connections.performance_client_secret_encrypted IS 'Encrypted Ozon Performance API client_secret (OAuth).';
