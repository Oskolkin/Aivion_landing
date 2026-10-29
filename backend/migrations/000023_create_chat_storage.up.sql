CREATE TABLE chat_sessions (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_message_at TIMESTAMPTZ,
    CONSTRAINT chk_chat_sessions_status CHECK (status IN ('active', 'archived'))
);

CREATE INDEX idx_chat_sessions_seller_status
    ON chat_sessions(seller_account_id, status);

CREATE INDEX idx_chat_sessions_seller_updated_desc
    ON chat_sessions(seller_account_id, updated_at DESC);

CREATE INDEX idx_chat_sessions_seller_last_message_desc
    ON chat_sessions(seller_account_id, last_message_at DESC);

CREATE INDEX idx_chat_sessions_user_created_desc
    ON chat_sessions(user_id, created_at DESC);

CREATE TABLE chat_messages (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    message_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_chat_messages_role CHECK (role IN ('user', 'assistant', 'system')),
    CONSTRAINT chk_chat_messages_type CHECK (message_type IN ('question', 'answer', 'error', 'meta'))
);

CREATE INDEX idx_chat_messages_seller_session_created_asc
    ON chat_messages(seller_account_id, session_id, created_at ASC);

CREATE INDEX idx_chat_messages_seller_created_desc
    ON chat_messages(seller_account_id, created_at DESC);

CREATE INDEX idx_chat_messages_session_created_asc
    ON chat_messages(session_id, created_at ASC);

CREATE TABLE chat_traces (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    user_message_id BIGINT REFERENCES chat_messages(id) ON DELETE SET NULL,
    assistant_message_id BIGINT REFERENCES chat_messages(id) ON DELETE SET NULL,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    planner_prompt_version TEXT NOT NULL,
    answer_prompt_version TEXT NOT NULL,
    planner_model TEXT NOT NULL,
    answer_model TEXT NOT NULL,
    detected_intent TEXT,
    tool_plan_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    validated_tool_plan_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    tool_results_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    fact_context_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_planner_response JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_answer_response JSONB NOT NULL DEFAULT '{}'::jsonb,
    answer_validation_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    estimated_cost NUMERIC(18,6) NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'running',
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_chat_traces_status CHECK (status IN ('running', 'completed', 'failed')),
    CONSTRAINT chk_chat_traces_input_tokens CHECK (input_tokens >= 0),
    CONSTRAINT chk_chat_traces_output_tokens CHECK (output_tokens >= 0),
    CONSTRAINT chk_chat_traces_estimated_cost CHECK (estimated_cost >= 0)
);

CREATE INDEX idx_chat_traces_seller_started_desc
    ON chat_traces(seller_account_id, started_at DESC);

CREATE INDEX idx_chat_traces_seller_status
    ON chat_traces(seller_account_id, status);

CREATE INDEX idx_chat_traces_seller_detected_intent
    ON chat_traces(seller_account_id, detected_intent);

CREATE INDEX idx_chat_traces_session_started_desc
    ON chat_traces(session_id, started_at DESC);

CREATE INDEX idx_chat_traces_user_message_id
    ON chat_traces(user_message_id);

CREATE INDEX idx_chat_traces_assistant_message_id
    ON chat_traces(assistant_message_id);

CREATE INDEX idx_chat_traces_tool_plan_payload_gin
    ON chat_traces USING GIN (tool_plan_payload);

CREATE INDEX idx_chat_traces_validated_tool_plan_payload_gin
    ON chat_traces USING GIN (validated_tool_plan_payload);

CREATE INDEX idx_chat_traces_fact_context_payload_gin
    ON chat_traces USING GIN (fact_context_payload);

CREATE INDEX idx_chat_traces_answer_validation_payload_gin
    ON chat_traces USING GIN (answer_validation_payload);

CREATE TABLE chat_feedback (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    message_id BIGINT NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    rating TEXT NOT NULL,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_chat_feedback_seller_message UNIQUE (seller_account_id, message_id),
    CONSTRAINT chk_chat_feedback_rating CHECK (rating IN ('positive', 'negative', 'neutral'))
);

CREATE INDEX idx_chat_feedback_seller_created_desc
    ON chat_feedback(seller_account_id, created_at DESC);

CREATE INDEX idx_chat_feedback_seller_rating
    ON chat_feedback(seller_account_id, rating);

CREATE INDEX idx_chat_feedback_session_created_desc
    ON chat_feedback(session_id, created_at DESC);

CREATE INDEX idx_chat_feedback_message_id
    ON chat_feedback(message_id);
