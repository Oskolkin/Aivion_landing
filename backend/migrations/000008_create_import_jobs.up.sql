CREATE TABLE import_jobs (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    sync_job_id BIGINT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'fetching', 'importing', 'completed', 'failed')),
    source_cursor TEXT,
    records_received INTEGER NOT NULL DEFAULT 0,
    records_imported INTEGER NOT NULL DEFAULT 0,
    records_failed INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_import_jobs_seller_account_id ON import_jobs(seller_account_id);
CREATE INDEX idx_import_jobs_sync_job_id ON import_jobs(sync_job_id);
CREATE INDEX idx_import_jobs_domain ON import_jobs(domain);
CREATE INDEX idx_import_jobs_status ON import_jobs(status);
CREATE INDEX idx_import_jobs_created_at ON import_jobs(created_at);