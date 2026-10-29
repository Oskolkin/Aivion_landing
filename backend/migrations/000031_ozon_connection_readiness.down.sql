ALTER TABLE ozon_connections
    DROP CONSTRAINT IF EXISTS ozon_connections_connection_status_check;

ALTER TABLE ozon_connections
    DROP COLUMN IF EXISTS performance_client_id_encrypted,
    DROP COLUMN IF EXISTS performance_client_secret_encrypted,
    DROP COLUMN IF EXISTS performance_token_expires_at,
    DROP COLUMN IF EXISTS seller_api_valid,
    DROP COLUMN IF EXISTS performance_api_valid,
    DROP COLUMN IF EXISTS connection_status,
    DROP COLUMN IF EXISTS connection_error_code,
    DROP COLUMN IF EXISTS connection_error_message,
    DROP COLUMN IF EXISTS disconnected_at,
    DROP COLUMN IF EXISTS retention_delete_after;
