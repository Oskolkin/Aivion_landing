CREATE TABLE alerts (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    alert_type TEXT NOT NULL,
    alert_group TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT,
    entity_sku BIGINT,
    entity_offer_id TEXT,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    severity TEXT NOT NULL,
    urgency TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    evidence_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    fingerprint TEXT NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_alerts_seller_fingerprint UNIQUE (seller_account_id, fingerprint),
    CONSTRAINT chk_alerts_group CHECK (
        alert_group IN ('sales', 'stock', 'advertising', 'price_economics')
    ),
    CONSTRAINT chk_alerts_entity_type CHECK (
        entity_type IN ('account', 'sku', 'product', 'campaign', 'pricing_constraint')
    ),
    CONSTRAINT chk_alerts_severity CHECK (
        severity IN ('low', 'medium', 'high', 'critical')
    ),
    CONSTRAINT chk_alerts_urgency CHECK (
        urgency IN ('low', 'medium', 'high', 'immediate')
    ),
    CONSTRAINT chk_alerts_status CHECK (
        status IN ('open', 'resolved', 'dismissed')
    )
);

CREATE INDEX idx_alerts_seller_status
    ON alerts(seller_account_id, status);

CREATE INDEX idx_alerts_seller_group_status
    ON alerts(seller_account_id, alert_group, status);

CREATE INDEX idx_alerts_seller_severity_status
    ON alerts(seller_account_id, severity, status);

CREATE INDEX idx_alerts_seller_entity_type
    ON alerts(seller_account_id, entity_type);

CREATE INDEX idx_alerts_seller_entity_sku
    ON alerts(seller_account_id, entity_sku);

CREATE INDEX idx_alerts_seller_last_seen_desc
    ON alerts(seller_account_id, last_seen_at DESC);

CREATE INDEX idx_alerts_evidence_payload_gin
    ON alerts USING GIN (evidence_payload);

CREATE TABLE alert_runs (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    run_type TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    sales_alerts_count INTEGER NOT NULL DEFAULT 0,
    stock_alerts_count INTEGER NOT NULL DEFAULT 0,
    ad_alerts_count INTEGER NOT NULL DEFAULT 0,
    price_alerts_count INTEGER NOT NULL DEFAULT 0,
    total_alerts_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_alert_runs_run_type CHECK (
        run_type IN ('manual', 'scheduled', 'post_sync', 'backfill')
    ),
    CONSTRAINT chk_alert_runs_status CHECK (
        status IN ('running', 'completed', 'failed')
    )
);

CREATE INDEX idx_alert_runs_seller_started_desc
    ON alert_runs(seller_account_id, started_at DESC);

CREATE INDEX idx_alert_runs_seller_status
    ON alert_runs(seller_account_id, status);

CREATE INDEX idx_alert_runs_seller_type_started_desc
    ON alert_runs(seller_account_id, run_type, started_at DESC);
