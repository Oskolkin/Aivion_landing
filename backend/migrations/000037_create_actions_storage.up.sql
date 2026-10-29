CREATE TABLE action_generation_runs (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    metric_date DATE NULL,
    status TEXT NOT NULL,
    context_hash TEXT NULL,
    final_status TEXT NULL,
    accepted_count INTEGER NOT NULL DEFAULT 0,
    repaired_count INTEGER NOT NULL DEFAULT 0,
    dropped_count INTEGER NOT NULL DEFAULT 0,
    fallback_count INTEGER NOT NULL DEFAULT 0,
    ai_focus_json JSONB NULL,
    top_action_keys_json JSONB NULL,
    data_quality_notes_json JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_action_generation_runs_seller_date
    ON action_generation_runs (seller_account_id, metric_date DESC, created_at DESC);

CREATE INDEX idx_action_generation_runs_seller_created
    ON action_generation_runs (seller_account_id, created_at DESC);

CREATE TABLE action_generation_category_runs (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES action_generation_runs(id) ON DELETE CASCADE,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    status TEXT NOT NULL,
    model TEXT NULL,
    input_context_json JSONB NULL,
    raw_response_json JSONB NULL,
    validation_json JSONB NULL,
    repair_response_json JSONB NULL,
    final_validation_json JSONB NULL,
    accepted_count INTEGER NOT NULL DEFAULT 0,
    repaired_count INTEGER NOT NULL DEFAULT 0,
    dropped_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_action_generation_category_runs_run
    ON action_generation_category_runs (run_id);

CREATE INDEX idx_action_generation_category_runs_seller_domain
    ON action_generation_category_runs (seller_account_id, domain, created_at DESC);

CREATE TABLE action_items (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    run_id BIGINT NULL REFERENCES action_generation_runs(id) ON DELETE SET NULL,

    action_key TEXT NOT NULL,
    type TEXT NOT NULL,
    category TEXT NOT NULL,
    priority TEXT NOT NULL,

    title TEXT NOT NULL,
    summary TEXT NULL,
    reason TEXT NOT NULL,
    next_step TEXT NOT NULL,
    expected_effect TEXT NULL,

    evidence_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    limitations_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    cta_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    affected_entities_json JSONB NOT NULL DEFAULT '{}'::jsonb,

    entity_group_ref TEXT NULL,
    source_candidate_ids_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    evidence_fact_ids_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,

    metric_date DATE NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (seller_account_id, action_key)
);

CREATE INDEX idx_action_items_seller_metric_date
    ON action_items (seller_account_id, metric_date DESC);

CREATE INDEX idx_action_items_seller_category
    ON action_items (seller_account_id, category);

CREATE INDEX idx_action_items_seller_priority
    ON action_items (seller_account_id, priority);

CREATE INDEX idx_action_items_run
    ON action_items (run_id);

CREATE TABLE action_user_states (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    action_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    checked_at TIMESTAMPTZ NULL,
    snoozed_until TIMESTAMPTZ NULL,
    dismissed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (seller_account_id, action_key)
);

CREATE INDEX idx_action_user_states_seller_status
    ON action_user_states (seller_account_id, status);

CREATE INDEX idx_action_user_states_seller_action_key
    ON action_user_states (seller_account_id, action_key);

CREATE TABLE action_entity_groups (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    metric_date DATE NULL,
    group_key TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_ids_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (seller_account_id, group_key)
);

CREATE INDEX idx_action_entity_groups_seller_date
    ON action_entity_groups (seller_account_id, metric_date DESC);

CREATE INDEX idx_action_entity_groups_seller_type
    ON action_entity_groups (seller_account_id, entity_type);
