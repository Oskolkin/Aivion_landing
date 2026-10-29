-- name: CreateOzonConnection :one
INSERT INTO ozon_connections (
    seller_account_id,
    client_id_encrypted,
    api_key_encrypted,
    status,
    last_check_at,
    last_check_result,
    last_error
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetOzonConnectionBySellerAccountID :one
SELECT * FROM ozon_connections
WHERE seller_account_id = $1
LIMIT 1;

-- name: GetOzonConnectionForOAuthRefresh :one
SELECT * FROM ozon_connections
WHERE seller_account_id = $1
FOR UPDATE;

-- name: UpsertOzonOAuthConnection :one
INSERT INTO ozon_connections (
  seller_account_id, client_id_encrypted, api_key_encrypted, status,
  auth_mode, oauth_access_token_encrypted, oauth_refresh_token_encrypted,
  oauth_access_token_expires_at, oauth_scopes, oauth_token_type,
  oauth_connection_state, oauth_token_updated_at,
  seller_api_valid, performance_api_valid, connection_status
) VALUES (
  $1,$2,$2,'valid','PRIVATE_OAUTH',$3,$4,$5,$6::jsonb,$7,'CONNECTED',NOW(),true,true,'connected'
)
ON CONFLICT ON CONSTRAINT ozon_connections_seller_account_id_key DO UPDATE SET
  auth_mode='PRIVATE_OAUTH',
  oauth_access_token_encrypted=EXCLUDED.oauth_access_token_encrypted,
  oauth_refresh_token_encrypted=EXCLUDED.oauth_refresh_token_encrypted,
  oauth_access_token_expires_at=EXCLUDED.oauth_access_token_expires_at,
  oauth_scopes=EXCLUDED.oauth_scopes,
  oauth_token_type=EXCLUDED.oauth_token_type,
  oauth_connection_state='CONNECTED',
  oauth_token_updated_at=NOW(),
  oauth_last_refresh_error=NULL,
  seller_api_valid=true,
  performance_api_valid=true,
  connection_status='connected',
  disconnected_at=NULL,
  retention_delete_after=NULL,
  updated_at=NOW()
RETURNING *;

-- name: UpdateOzonOAuthTokens :one
UPDATE ozon_connections SET
  oauth_access_token_encrypted=$2,
  oauth_refresh_token_encrypted=$3,
  oauth_access_token_expires_at=$4,
  oauth_scopes=$5::jsonb,
  oauth_token_type=$6,
  oauth_connection_state='CONNECTED',
  oauth_token_updated_at=NOW(),
  oauth_last_refresh_at=NOW(),
  oauth_last_refresh_error=NULL,
  seller_api_valid=true,
  performance_api_valid=true,
  connection_status='connected',
  updated_at=NOW()
WHERE seller_account_id=$1 AND auth_mode='PRIVATE_OAUTH'
RETURNING *;

-- name: RecordOzonOAuthRefreshError :one
UPDATE ozon_connections SET
  oauth_connection_state='REFRESH_ERROR', oauth_last_refresh_at=NOW(),
  oauth_last_refresh_error=$2, updated_at=NOW()
WHERE seller_account_id=$1 AND auth_mode='PRIVATE_OAUTH'
RETURNING *;

-- name: UpdateOzonConnectionCredentials :one
UPDATE ozon_connections
SET
    client_id_encrypted = $2,
    api_key_encrypted = $3,
    status = $4,
    last_check_at = $5,
    last_check_result = $6,
    last_error = $7,
    updated_at = NOW()
WHERE seller_account_id = $1
RETURNING *;

-- name: UpdateOzonConnectionCheckResult :one
UPDATE ozon_connections
SET
    status = $2,
    last_check_at = $3,
    last_check_result = $4,
    last_error = $5,
    updated_at = NOW()
WHERE seller_account_id = $1
RETURNING *;

-- name: UpdateOzonConnectionStatus :one
UPDATE ozon_connections
SET
    status = $2,
    updated_at = NOW()
WHERE seller_account_id = $1
RETURNING *;

-- name: UpdateOzonPerformanceBearerToken :one
UPDATE ozon_connections
SET
    performance_token_encrypted = $2,
    performance_status = 'unknown',
    performance_last_check_at = NULL,
    performance_last_check_result = NULL,
    performance_last_error = NULL,
    updated_at = NOW()
WHERE seller_account_id = $1
RETURNING *;

-- name: ClearOzonPerformanceBearerToken :one
UPDATE ozon_connections
SET
    performance_token_encrypted = NULL,
    performance_status = 'not_configured',
    performance_last_check_at = NULL,
    performance_last_check_result = NULL,
    performance_last_error = NULL,
    updated_at = NOW()
WHERE seller_account_id = $1
RETURNING *;

-- name: UpdateOzonPerformanceCheckResult :one
UPDATE ozon_connections
SET
    performance_status = $2,
    performance_last_check_at = $3,
    performance_last_check_result = $4,
    performance_last_error = $5,
    updated_at = NOW()
WHERE seller_account_id = $1
RETURNING *;

-- name: UpsertOzonConnectionValidated :one
INSERT INTO ozon_connections (
    seller_account_id,
    client_id_encrypted,
    api_key_encrypted,
    status,
    last_check_at,
    last_check_result,
    last_error,
    performance_client_id_encrypted,
    performance_client_secret_encrypted,
    performance_token_encrypted,
    performance_token_expires_at,
    performance_status,
    performance_last_check_at,
    performance_last_check_result,
    performance_last_error,
    seller_api_valid,
    performance_api_valid,
    connection_status,
    connection_error_code,
    connection_error_message,
    disconnected_at,
    retention_delete_after
) VALUES (
    $1, $2, $3, 'valid', NOW(), 'valid', NULL,
    $4, $5, $6, $7, 'valid', NOW(), 'valid', NULL,
    true, true, 'connected', NULL, NULL, NULL, NULL
)
ON CONFLICT ON CONSTRAINT ozon_connections_seller_account_id_key DO UPDATE SET
    client_id_encrypted = EXCLUDED.client_id_encrypted,
    api_key_encrypted = EXCLUDED.api_key_encrypted,
    status = 'valid',
    last_check_at = NOW(),
    last_check_result = 'valid',
    last_error = NULL,
    performance_client_id_encrypted = EXCLUDED.performance_client_id_encrypted,
    performance_client_secret_encrypted = EXCLUDED.performance_client_secret_encrypted,
    performance_token_encrypted = EXCLUDED.performance_token_encrypted,
    performance_token_expires_at = EXCLUDED.performance_token_expires_at,
    performance_status = 'valid',
    performance_last_check_at = NOW(),
    performance_last_check_result = 'valid',
    performance_last_error = NULL,
    seller_api_valid = true,
    performance_api_valid = true,
    connection_status = 'connected',
    connection_error_code = NULL,
    connection_error_message = NULL,
    disconnected_at = NULL,
    retention_delete_after = NULL,
    updated_at = NOW()
RETURNING *;

-- name: UpdateOzonConnectionValidationErrors :one
UPDATE ozon_connections
SET
    seller_api_valid = $2,
    performance_api_valid = $3,
    connection_status = $4,
    connection_error_code = $5,
    connection_error_message = $6,
    updated_at = NOW()
WHERE seller_account_id = $1
RETURNING *;

-- name: DisconnectOzonConnection :one
UPDATE ozon_connections
SET
    client_id_encrypted = $2,
    api_key_encrypted = $3,
    status = 'invalid',
    last_check_at = NULL,
    last_check_result = NULL,
    last_error = NULL,
    performance_client_id_encrypted = NULL,
    performance_client_secret_encrypted = NULL,
    performance_token_encrypted = NULL,
    performance_token_expires_at = NULL,
    performance_status = 'not_configured',
    performance_last_check_at = NULL,
    performance_last_check_result = NULL,
    performance_last_error = NULL,
    seller_api_valid = false,
    performance_api_valid = false,
    connection_status = 'disconnected_retention',
    connection_error_code = NULL,
    connection_error_message = NULL,
    disconnected_at = $4,
    retention_delete_after = $5,
    auth_mode = 'LEGACY',
    oauth_access_token_encrypted = NULL,
    oauth_refresh_token_encrypted = NULL,
    oauth_access_token_expires_at = NULL,
    oauth_scopes = '[]'::jsonb,
    oauth_token_type = NULL,
    oauth_connection_state = 'DISCONNECTED',
    oauth_token_updated_at = NULL,
    oauth_last_refresh_at = NULL,
    oauth_last_refresh_error = NULL,
    updated_at = NOW()
WHERE seller_account_id = $1
RETURNING *;

-- name: ListOzonConnectionsForPeriodicSync :many
SELECT seller_account_id
FROM ozon_connections
WHERE connection_status = 'connected'
  AND seller_api_valid = true
  AND performance_api_valid = true
  AND auth_mode = 'LEGACY';

-- name: ListOzonConnectionsDueForRetentionCleanup :many
SELECT seller_account_id, retention_delete_after
FROM ozon_connections
WHERE connection_status = 'disconnected_retention'
  AND retention_delete_after IS NOT NULL
  AND retention_delete_after <= NOW();
