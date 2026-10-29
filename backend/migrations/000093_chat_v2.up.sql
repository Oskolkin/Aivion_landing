-- Prompt 15: Chat v2 turn/context/grounding/action-binding model.
-- Legacy chat_sessions.action_key is retained until Prompt 17.

CREATE TABLE chat_turns (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    user_message_id BIGINT NOT NULL REFERENCES chat_messages(id) ON DELETE RESTRICT,
    assistant_message_id BIGINT REFERENCES chat_messages(id) ON DELETE SET NULL,
    answer_mode TEXT NOT NULL DEFAULT 'GENERAL_GUIDANCE',
    context_mode TEXT NOT NULL,
    status TEXT NOT NULL,
    publication_id BIGINT REFERENCES analysis_publications(id) ON DELETE RESTRICT,
    factual_universe_id UUID,
    factual_context_hash TEXT,
    model_policy_version TEXT NOT NULL,
    tool_policy_version TEXT NOT NULL,
    memory_policy_version TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    result_schema_version TEXT NOT NULL,
    grounding_status TEXT NOT NULL DEFAULT 'NONE',
    result_hash TEXT,
    referenced_entities_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    limitations_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_chat_turns_context_mode CHECK (context_mode IN ('PUBLICATION_BOUND', 'PREPUBLICATION_FACTUAL', 'GENERAL')),
    CONSTRAINT chk_chat_turns_status CHECK (status IN (
        'QUEUED', 'RESOLVING_CONTEXT', 'READING_DATA', 'ANALYZING', 'VALIDATING', 'STREAMING', 'COMPLETED', 'FAILED', 'CANCELLED'
    )),
    CONSTRAINT chk_chat_turns_grounding CHECK (grounding_status IN ('NONE', 'GROUNDED', 'LIMITED')),
    CONSTRAINT chk_chat_turns_pub_bound CHECK (
        (context_mode = 'PUBLICATION_BOUND' AND publication_id IS NOT NULL AND factual_universe_id IS NOT NULL)
        OR (context_mode <> 'PUBLICATION_BOUND')
    )
);

CREATE INDEX idx_chat_turns_session_started ON chat_turns (session_id, started_at DESC);
CREATE INDEX idx_chat_turns_seller_started ON chat_turns (seller_account_id, started_at DESC);
CREATE UNIQUE INDEX uq_chat_turns_user_message ON chat_turns (user_message_id);

CREATE TABLE chat_turn_contexts (
    turn_id BIGINT PRIMARY KEY REFERENCES chat_turns(id) ON DELETE CASCADE,
    analysis_date DATE,
    analysis_cutoff_at TIMESTAMPTZ,
    publication_changed_from_id BIGINT REFERENCES analysis_publications(id) ON DELETE SET NULL,
    base_context_hash TEXT NOT NULL,
    conversation_context_hash TEXT NOT NULL,
    attachment_context_hash TEXT NOT NULL,
    action_context_hash TEXT NOT NULL,
    context_truncated BOOLEAN NOT NULL DEFAULT FALSE,
    truncation_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE chat_turn_tool_calls (
    id BIGSERIAL PRIMARY KEY,
    call_id TEXT NOT NULL,
    turn_id BIGINT NOT NULL REFERENCES chat_turns(id) ON DELETE CASCADE,
    round_no INTEGER NOT NULL,
    ordinal INTEGER NOT NULL,
    tool_key TEXT NOT NULL,
    tool_version TEXT NOT NULL,
    args_hash TEXT NOT NULL,
    status TEXT NOT NULL,
    result_hash TEXT,
    fact_handles_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error_code TEXT,
    CONSTRAINT uq_chat_turn_tool_calls_call UNIQUE (turn_id, call_id),
    CONSTRAINT chk_chat_turn_tool_calls_status CHECK (status IN ('STARTED', 'SUCCEEDED', 'FAILED', 'REJECTED', 'BUDGET_EXCEEDED'))
);

CREATE INDEX idx_chat_turn_tool_calls_turn ON chat_turn_tool_calls (turn_id, round_no, ordinal);

CREATE TABLE chat_turn_grounding_refs (
    id BIGSERIAL PRIMARY KEY,
    turn_id BIGINT NOT NULL REFERENCES chat_turns(id) ON DELETE CASCADE,
    handle TEXT NOT NULL,
    ref_type TEXT NOT NULL,
    ref_id TEXT NOT NULL,
    fingerprint TEXT NOT NULL DEFAULT '',
    ordinal INTEGER NOT NULL,
    CONSTRAINT uq_chat_turn_grounding_handle UNIQUE (turn_id, handle),
    CONSTRAINT chk_chat_turn_grounding_type CHECK (ref_type IN (
        'FACT', 'FINDING', 'RECOMMENDATION', 'ACTION', 'ACTION_CASE', 'ATTACHMENT', 'PUBLICATION'
    ))
);

CREATE INDEX idx_chat_turn_grounding_turn ON chat_turn_grounding_refs (turn_id, ordinal);

CREATE TABLE chat_action_bindings (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    session_id BIGINT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    origin_publication_id BIGINT NOT NULL REFERENCES analysis_publications(id) ON DELETE RESTRICT,
    origin_kind TEXT NOT NULL,
    origin_group_id UUID,
    origin_case_id UUID,
    origin_recommendation_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
    origin_member_count INTEGER NOT NULL DEFAULT 1,
    fingerprint TEXT NOT NULL,
    title TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_chat_action_bindings_session UNIQUE (session_id),
    CONSTRAINT chk_chat_action_bindings_kind CHECK (origin_kind IN ('GROUP', 'CASE')),
    CONSTRAINT chk_chat_action_bindings_origin CHECK (
        (origin_kind = 'GROUP' AND origin_group_id IS NOT NULL)
        OR (origin_kind = 'CASE' AND origin_case_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX uq_chat_action_bindings_fingerprint
    ON chat_action_bindings (seller_account_id, fingerprint);

CREATE TABLE chat_action_binding_members (
    id BIGSERIAL PRIMARY KEY,
    binding_id BIGINT NOT NULL REFERENCES chat_action_bindings(id) ON DELETE CASCADE,
    case_id UUID NOT NULL,
    member_order INTEGER NOT NULL,
    target_kind TEXT NOT NULL DEFAULT '',
    target_id TEXT NOT NULL DEFAULT '',
    action_type TEXT NOT NULL DEFAULT '',
    CONSTRAINT uq_chat_action_binding_members UNIQUE (binding_id, case_id)
);

CREATE INDEX idx_chat_action_binding_members_binding ON chat_action_binding_members (binding_id, member_order);

CREATE TABLE chat_export_provenance (
    id BIGSERIAL PRIMARY KEY,
    turn_id BIGINT REFERENCES chat_turns(id) ON DELETE SET NULL,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    attachment_id UUID REFERENCES chat_attachments(id) ON DELETE SET NULL,
    publication_id BIGINT REFERENCES analysis_publications(id) ON DELETE SET NULL,
    factual_universe_id UUID,
    factual_context_hash TEXT NOT NULL,
    query_spec_json JSONB NOT NULL,
    period_from DATE,
    period_to DATE,
    filters_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    export_schema_version TEXT NOT NULL,
    artifact_hash TEXT NOT NULL,
    attachment_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_export_provenance_seller ON chat_export_provenance (seller_account_id, created_at DESC);

ALTER TABLE chat_attachments
    ADD COLUMN IF NOT EXISTS completeness_status TEXT;

ALTER TABLE chat_attachments
    DROP CONSTRAINT IF EXISTS chat_attachments_completeness_status_chk;

ALTER TABLE chat_attachments
    ADD CONSTRAINT chat_attachments_completeness_status_chk CHECK (
        completeness_status IS NULL OR completeness_status IN (
            'COMPLETE', 'PARTIAL', 'UNREADABLE', 'VISION_REQUIRED'
        )
    );

ALTER TABLE chat_request_idempotency
    ADD COLUMN IF NOT EXISTS turn_id BIGINT REFERENCES chat_turns(id) ON DELETE SET NULL;

ALTER TABLE chat_request_idempotency
    ADD COLUMN IF NOT EXISTS binding_id BIGINT REFERENCES chat_action_bindings(id) ON DELETE SET NULL;
