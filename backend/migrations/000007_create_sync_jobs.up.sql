CREATE TABLE sync_jobs (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    error_message TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT sync_jobs_type_check CHECK (type IN ('initial_sync')),
    CONSTRAINT sync_jobs_status_check CHECK (status IN ('pending', 'running', 'completed', 'failed'))
);

CREATE INDEX idx_sync_jobs_seller_account_id ON sync_jobs(seller_account_id);
CREATE INDEX idx_sync_jobs_status ON sync_jobs(status);