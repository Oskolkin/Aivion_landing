ALTER TABLE ozon_connections
    DROP CONSTRAINT IF EXISTS ozon_connections_performance_status_check,
    DROP COLUMN IF EXISTS performance_last_error,
    DROP COLUMN IF EXISTS performance_last_check_result,
    DROP COLUMN IF EXISTS performance_last_check_at,
    DROP COLUMN IF EXISTS performance_status,
    DROP COLUMN IF EXISTS performance_token_encrypted;
