CREATE TABLE recommendations (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    source TEXT NOT NULL DEFAULT 'chatgpt',
    recommendation_type TEXT NOT NULL,
    horizon TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT,
    entity_sku BIGINT,
    entity_offer_id TEXT,
    title TEXT NOT NULL,
    what_happened TEXT NOT NULL,
    why_it_matters TEXT NOT NULL,
    recommended_action TEXT NOT NULL,
    expected_effect TEXT,
    priority_score NUMERIC(8,2) NOT NULL DEFAULT 0,
    priority_level TEXT NOT NULL,
    urgency TEXT NOT NULL,
    confidence_level TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    supporting_metrics_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    constraints_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    ai_model TEXT,
    ai_prompt_version TEXT,
    raw_ai_response JSONB,
    fingerprint TEXT NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    dismissed_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_recommendations_seller_fingerprint UNIQUE (seller_account_id, fingerprint),
    CONSTRAINT chk_recommendations_source CHECK (
        source IN ('chatgpt', 'manual', 'system')
    ),
    CONSTRAINT chk_recommendations_horizon CHECK (
        horizon IN ('short_term', 'medium_term', 'long_term')
    ),
    CONSTRAINT chk_recommendations_entity_type CHECK (
        entity_type IN ('account', 'sku', 'product', 'campaign', 'pricing_constraint')
    ),
    CONSTRAINT chk_recommendations_priority_level CHECK (
        priority_level IN ('low', 'medium', 'high', 'critical')
    ),
    CONSTRAINT chk_recommendations_urgency CHECK (
        urgency IN ('low', 'medium', 'high', 'immediate')
    ),
    CONSTRAINT chk_recommendations_confidence_level CHECK (
        confidence_level IN ('low', 'medium', 'high')
    ),
    CONSTRAINT chk_recommendations_status CHECK (
        status IN ('open', 'accepted', 'dismissed', 'resolved')
    ),
    CONSTRAINT chk_recommendations_priority_score CHECK (
        priority_score >= 0 AND priority_score <= 100
    )
);

CREATE INDEX idx_recommendations_seller_status
    ON recommendations(seller_account_id, status);

CREATE INDEX idx_recommendations_seller_priority_level_status
    ON recommendations(seller_account_id, priority_level, status);

CREATE INDEX idx_recommendations_seller_confidence_level_status
    ON recommendations(seller_account_id, confidence_level, status);

CREATE INDEX idx_recommendations_seller_horizon_status
    ON recommendations(seller_account_id, horizon, status);

CREATE INDEX idx_recommendations_seller_entity_type
    ON recommendations(seller_account_id, entity_type);

CREATE INDEX idx_recommendations_seller_entity_sku
    ON recommendations(seller_account_id, entity_sku);

CREATE INDEX idx_recommendations_seller_last_seen_desc
    ON recommendations(seller_account_id, last_seen_at DESC);

CREATE INDEX idx_recommendations_seller_priority_score_desc
    ON recommendations(seller_account_id, priority_score DESC);

CREATE INDEX idx_recommendations_supporting_metrics_payload_gin
    ON recommendations USING GIN (supporting_metrics_payload);

CREATE INDEX idx_recommendations_constraints_payload_gin
    ON recommendations USING GIN (constraints_payload);

CREATE TABLE recommendation_alert_links (
    recommendation_id BIGINT NOT NULL REFERENCES recommendations(id) ON DELETE CASCADE,
    alert_id BIGINT NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    link_type TEXT NOT NULL DEFAULT 'supporting_signal',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (recommendation_id, alert_id),
    CONSTRAINT chk_recommendation_alert_links_link_type CHECK (
        link_type IN ('supporting_signal', 'primary_signal', 'related_signal')
    )
);

CREATE INDEX idx_recommendation_alert_links_seller_recommendation
    ON recommendation_alert_links(seller_account_id, recommendation_id);

CREATE INDEX idx_recommendation_alert_links_seller_alert
    ON recommendation_alert_links(seller_account_id, alert_id);

CREATE INDEX idx_recommendation_alert_links_seller_link_type
    ON recommendation_alert_links(seller_account_id, link_type);

CREATE TABLE recommendation_runs (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    run_type TEXT NOT NULL,
    status TEXT NOT NULL,
    as_of_date DATE,
    ai_model TEXT,
    ai_prompt_version TEXT,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    estimated_cost NUMERIC(12,6) NOT NULL DEFAULT 0,
    generated_recommendations_count INTEGER NOT NULL DEFAULT 0,
    accepted_recommendations_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_recommendation_runs_run_type CHECK (
        run_type IN ('manual', 'scheduled', 'post_alerts', 'backfill')
    ),
    CONSTRAINT chk_recommendation_runs_status CHECK (
        status IN ('running', 'completed', 'failed')
    ),
    CONSTRAINT chk_recommendation_runs_input_tokens CHECK (input_tokens >= 0),
    CONSTRAINT chk_recommendation_runs_output_tokens CHECK (output_tokens >= 0),
    CONSTRAINT chk_recommendation_runs_estimated_cost CHECK (estimated_cost >= 0),
    CONSTRAINT chk_recommendation_runs_generated_recommendations_count CHECK (generated_recommendations_count >= 0),
    CONSTRAINT chk_recommendation_runs_accepted_recommendations_count CHECK (accepted_recommendations_count >= 0)
);

CREATE INDEX idx_recommendation_runs_seller_started_desc
    ON recommendation_runs(seller_account_id, started_at DESC);

CREATE INDEX idx_recommendation_runs_seller_status
    ON recommendation_runs(seller_account_id, status);

CREATE INDEX idx_recommendation_runs_seller_type_started_desc
    ON recommendation_runs(seller_account_id, run_type, started_at DESC);

CREATE INDEX idx_recommendation_runs_seller_as_of_date_desc
    ON recommendation_runs(seller_account_id, as_of_date DESC);
