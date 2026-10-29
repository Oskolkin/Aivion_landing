-- Temporary external auditor review invites + read-only sessions.
ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS read_only BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS session_kind TEXT NOT NULL DEFAULT 'user',
    ADD COLUMN IF NOT EXISTS review_invite_id BIGINT NULL;

CREATE TABLE IF NOT EXISTS review_invites (
    id BIGSERIAL PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    label TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_review_invites_expires_at ON review_invites (expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_review_invite_id ON sessions (review_invite_id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'sessions_review_invite_id_fkey'
    ) THEN
        ALTER TABLE sessions
            ADD CONSTRAINT sessions_review_invite_id_fkey
            FOREIGN KEY (review_invite_id) REFERENCES review_invites(id) ON DELETE SET NULL;
    END IF;
END $$;
