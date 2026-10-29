CREATE TABLE seller_accounts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'onboarding',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT seller_accounts_status_check CHECK (status IN ('onboarding', 'active', 'archived'))
);

CREATE INDEX idx_seller_accounts_user_id ON seller_accounts(user_id);