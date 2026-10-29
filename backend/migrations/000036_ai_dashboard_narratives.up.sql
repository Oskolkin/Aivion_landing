CREATE TABLE ai_dashboard_narratives (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts (id) ON DELETE CASCADE,
    metric_date DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    health_score INTEGER,
    severity TEXT,
    risk_drivers_hash TEXT NOT NULL DEFAULT '',
    context_hash TEXT NOT NULL DEFAULT '',
    prompt_version TEXT NOT NULL,
    model TEXT,
    raw_ai_response TEXT,
    validated_payload JSONB,
    validation_errors JSONB NOT NULL DEFAULT '[]'::jsonb,
    input_tokens INTEGER,
    output_tokens INTEGER,
    estimated_cost NUMERIC(18, 6),
    error_message TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_ai_dashboard_narratives_status CHECK (
        status IN ('pending', 'running', 'completed', 'failed', 'fallback')
    ),
    CONSTRAINT chk_ai_dashboard_narratives_severity CHECK (
        severity IS NULL
        OR severity IN ('low', 'attention', 'medium', 'high', 'critical')
    ),
    CONSTRAINT chk_ai_dashboard_narratives_health_score CHECK (
        health_score IS NULL
        OR (health_score >= 0 AND health_score <= 100)
    ),
    CONSTRAINT uq_ai_dashboard_narratives_seller_date UNIQUE (seller_account_id, metric_date)
);

CREATE INDEX idx_ai_dashboard_narratives_seller_metric
    ON ai_dashboard_narratives (seller_account_id, metric_date DESC);

CREATE INDEX idx_ai_dashboard_narratives_seller_status_metric
    ON ai_dashboard_narratives (seller_account_id, status, metric_date DESC);
