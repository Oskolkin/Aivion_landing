-- Global Ozon description category dictionary (tree is marketplace-wide).
CREATE TABLE description_categories (
    description_category_id BIGINT PRIMARY KEY,
    parent_id BIGINT,
    category_name TEXT NOT NULL,
    path TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_description_categories_parent_id ON description_categories (parent_id);

CREATE TABLE ai_daily_summary_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts (id) ON DELETE CASCADE,
    metric_date DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    account_summary_id UUID,
    category_total_count INTEGER NOT NULL DEFAULT 0,
    category_completed_count INTEGER NOT NULL DEFAULT 0,
    category_failed_count INTEGER NOT NULL DEFAULT 0,
    prompt_version TEXT NOT NULL,
    model TEXT,
    estimated_input_tokens INTEGER,
    estimated_output_tokens INTEGER,
    estimated_cost NUMERIC(18, 6),
    error_message TEXT,
    data_quality_notes JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT chk_ai_daily_summary_runs_status CHECK (
        status IN ('pending', 'running', 'completed', 'partial', 'failed')
    ),
    CONSTRAINT uq_ai_daily_summary_runs_seller_date UNIQUE (seller_account_id, metric_date)
);

CREATE INDEX idx_ai_daily_summary_runs_seller_metric ON ai_daily_summary_runs (seller_account_id, metric_date DESC);

CREATE TABLE ai_daily_summaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID REFERENCES ai_daily_summary_runs (id) ON DELETE SET NULL,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts (id) ON DELETE CASCADE,
    metric_date DATE NOT NULL,
    summary_type TEXT NOT NULL,
    scope_type TEXT NOT NULL,
    scope_id TEXT NOT NULL DEFAULT '',
    scope_name TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    health_score INTEGER,
    severity TEXT,
    data_richness TEXT,
    summary_depth TEXT,
    headline TEXT,
    executive_summary TEXT,
    summary_json JSONB,
    input_context_json JSONB,
    previous_summary_id UUID REFERENCES ai_daily_summaries (id) ON DELETE SET NULL,
    linked_alert_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    linked_recommendation_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    prompt_version TEXT NOT NULL,
    model TEXT,
    estimated_input_tokens INTEGER,
    estimated_output_tokens INTEGER,
    estimated_cost NUMERIC(18, 6),
    validation_errors JSONB,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_ai_daily_summaries_summary_type CHECK (
        summary_type IN ('account_daily_summary', 'category_daily_summary')
    ),
    CONSTRAINT chk_ai_daily_summaries_scope_type CHECK (scope_type IN ('account', 'category')),
    CONSTRAINT chk_ai_daily_summaries_status CHECK (
        status IN ('pending', 'completed', 'failed', 'not_enough_data', 'running')
    ),
    CONSTRAINT chk_ai_daily_summaries_severity CHECK (
        severity IS NULL
        OR severity IN ('low', 'medium', 'high', 'critical')
    ),
    CONSTRAINT chk_ai_daily_summaries_data_richness CHECK (
        data_richness IS NULL
        OR data_richness IN ('low', 'medium', 'high')
    ),
    CONSTRAINT chk_ai_daily_summaries_summary_depth CHECK (
        summary_depth IS NULL
        OR summary_depth IN ('short', 'normal', 'deep')
    ),
    CONSTRAINT chk_ai_daily_summaries_health_score CHECK (
        health_score IS NULL
        OR (health_score >= 0 AND health_score <= 100)
    ),
    CONSTRAINT uq_ai_daily_summaries_scope UNIQUE (seller_account_id, metric_date, summary_type, scope_id)
);

CREATE INDEX idx_ai_daily_summaries_seller_metric ON ai_daily_summaries (seller_account_id, metric_date DESC);
CREATE INDEX idx_ai_daily_summaries_seller_type_metric ON ai_daily_summaries (seller_account_id, summary_type, metric_date DESC);
CREATE INDEX idx_ai_daily_summaries_seller_scope_metric ON ai_daily_summaries (seller_account_id, scope_type, scope_id, metric_date DESC);

ALTER TABLE ai_daily_summary_runs
    ADD CONSTRAINT fk_ai_daily_summary_runs_account_summary
        FOREIGN KEY (account_summary_id) REFERENCES ai_daily_summaries (id) ON DELETE SET NULL;
