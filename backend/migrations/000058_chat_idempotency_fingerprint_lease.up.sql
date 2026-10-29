-- The uniqueness scope intentionally remains seller_account_id + client_request_id.
-- user_id is retained on the row and validated by the application before replaying
-- or reclaiming a request, preventing one seller user from replaying another's key.
ALTER TABLE chat_request_idempotency
    ADD COLUMN IF NOT EXISTS request_hash TEXT,
    ADD COLUMN IF NOT EXISTS request_type TEXT,
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS lease_until TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS owner_id TEXT;

CREATE INDEX IF NOT EXISTS chat_request_idempotency_running_lease_idx
    ON chat_request_idempotency (seller_account_id, lease_until)
    WHERE status = 'running';
