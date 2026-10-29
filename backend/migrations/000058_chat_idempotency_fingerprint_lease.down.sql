DROP INDEX IF EXISTS chat_request_idempotency_running_lease_idx;

ALTER TABLE chat_request_idempotency
    DROP COLUMN IF EXISTS owner_id,
    DROP COLUMN IF EXISTS lease_until,
    DROP COLUMN IF EXISTS heartbeat_at,
    DROP COLUMN IF EXISTS started_at,
    DROP COLUMN IF EXISTS request_type,
    DROP COLUMN IF EXISTS request_hash;
