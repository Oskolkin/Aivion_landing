CREATE TABLE raw_payloads (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    import_job_id BIGINT NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    source TEXT NOT NULL,
    request_key TEXT,
    storage_bucket TEXT NOT NULL,
    storage_object_key TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_raw_payloads_seller_account_id ON raw_payloads(seller_account_id);
CREATE INDEX idx_raw_payloads_import_job_id ON raw_payloads(import_job_id);
CREATE INDEX idx_raw_payloads_domain ON raw_payloads(domain);
CREATE INDEX idx_raw_payloads_received_at ON raw_payloads(received_at);
CREATE INDEX idx_raw_payloads_payload_hash ON raw_payloads(payload_hash);