CREATE TABLE sync_cursors (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    cursor_type TEXT NOT NULL,
    cursor_value TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_sync_cursors_seller_account_domain_cursor_type
    ON sync_cursors(seller_account_id, domain, cursor_type);

CREATE INDEX idx_sync_cursors_seller_account_id ON sync_cursors(seller_account_id);
CREATE INDEX idx_sync_cursors_domain ON sync_cursors(domain);
CREATE INDEX idx_sync_cursors_updated_at ON sync_cursors(updated_at);