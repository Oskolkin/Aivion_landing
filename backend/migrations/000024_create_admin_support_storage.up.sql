CREATE TABLE admin_action_logs (
    id BIGSERIAL PRIMARY KEY,
    admin_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    admin_email TEXT NOT NULL,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    action_type TEXT NOT NULL,
    target_type TEXT,
    target_id BIGINT,
    request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    result_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'running',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    CONSTRAINT chk_admin_action_logs_action_type CHECK (
        action_type IN (
            'rerun_sync',
            'reset_cursor',
            'rerun_metrics',
            'rerun_alerts',
            'rerun_recommendations',
            'update_billing_state',
            'view_raw_ai_payload'
        )
    ),
    CONSTRAINT chk_admin_action_logs_status CHECK (
        status IN ('running', 'completed', 'failed')
    )
);

CREATE INDEX idx_admin_action_logs_seller_created
    ON admin_action_logs (seller_account_id, created_at DESC);

CREATE INDEX idx_admin_action_logs_admin_created
    ON admin_action_logs (admin_user_id, created_at DESC);

CREATE INDEX idx_admin_action_logs_action_created
    ON admin_action_logs (action_type, created_at DESC);

CREATE INDEX idx_admin_action_logs_status_created
    ON admin_action_logs (status, created_at DESC);

CREATE INDEX idx_admin_action_logs_target
    ON admin_action_logs (target_type, target_id);

CREATE INDEX idx_admin_action_logs_request_payload_gin
    ON admin_action_logs USING GIN (request_payload);

CREATE INDEX idx_admin_action_logs_result_payload_gin
    ON admin_action_logs USING GIN (result_payload);

CREATE TABLE seller_billing_state (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL UNIQUE REFERENCES seller_accounts(id) ON DELETE CASCADE,
    plan_code TEXT NOT NULL DEFAULT 'internal',
    status TEXT NOT NULL DEFAULT 'internal',
    trial_ends_at TIMESTAMPTZ,
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    ai_tokens_limit_month BIGINT,
    ai_tokens_used_month BIGINT NOT NULL DEFAULT 0,
    estimated_ai_cost_month NUMERIC(18,6) NOT NULL DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_seller_billing_state_status CHECK (
        status IN ('trial', 'active', 'past_due', 'paused', 'cancelled', 'internal')
    ),
    CONSTRAINT chk_seller_billing_state_ai_tokens_limit_month CHECK (
        ai_tokens_limit_month IS NULL OR ai_tokens_limit_month >= 0
    ),
    CONSTRAINT chk_seller_billing_state_ai_tokens_used_month CHECK (
        ai_tokens_used_month >= 0
    ),
    CONSTRAINT chk_seller_billing_state_estimated_ai_cost_month CHECK (
        estimated_ai_cost_month >= 0
    )
);

CREATE INDEX idx_seller_billing_state_status
    ON seller_billing_state (status);

CREATE INDEX idx_seller_billing_state_period_end
    ON seller_billing_state (current_period_end);

CREATE INDEX idx_seller_billing_state_updated
    ON seller_billing_state (updated_at DESC);

CREATE TABLE recommendation_run_diagnostics (
    id BIGSERIAL PRIMARY KEY,
    recommendation_run_id BIGINT REFERENCES recommendation_runs(id) ON DELETE CASCADE,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    openai_request_id TEXT,
    ai_model TEXT,
    prompt_version TEXT,
    context_payload_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_openai_response JSONB NOT NULL DEFAULT '{}'::jsonb,
    validation_result_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    rejected_items_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_stage TEXT,
    error_message TEXT,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    estimated_cost NUMERIC(18,6) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_recommendation_run_diagnostics_input_tokens CHECK (input_tokens >= 0),
    CONSTRAINT chk_recommendation_run_diagnostics_output_tokens CHECK (output_tokens >= 0),
    CONSTRAINT chk_recommendation_run_diagnostics_estimated_cost CHECK (estimated_cost >= 0)
);

CREATE INDEX idx_recommendation_run_diagnostics_run
    ON recommendation_run_diagnostics (recommendation_run_id);

CREATE INDEX idx_recommendation_run_diagnostics_seller_created
    ON recommendation_run_diagnostics (seller_account_id, created_at DESC);

CREATE INDEX idx_recommendation_run_diagnostics_error_stage
    ON recommendation_run_diagnostics (seller_account_id, error_stage, created_at DESC);

CREATE INDEX idx_recommendation_run_diagnostics_context_gin
    ON recommendation_run_diagnostics USING GIN (context_payload_summary);

CREATE INDEX idx_recommendation_run_diagnostics_validation_gin
    ON recommendation_run_diagnostics USING GIN (validation_result_payload);

CREATE INDEX idx_recommendation_run_diagnostics_rejected_gin
    ON recommendation_run_diagnostics USING GIN (rejected_items_payload);
