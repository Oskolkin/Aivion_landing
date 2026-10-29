-- Prompt №13: canonical Action Case persistence (not legacy action_items).

CREATE TABLE action_signatures (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE RESTRICT,
    signature_hash TEXT NOT NULL CHECK (signature_hash ~ '^[0-9a-f]{64}$'),
    action_type TEXT NOT NULL CHECK (action_type <> ''),
    target_kind TEXT NOT NULL CHECK (target_kind <> ''),
    target_id TEXT NOT NULL CHECK (target_id <> ''),
    normalized_context_hash TEXT NOT NULL CHECK (normalized_context_hash ~ '^[0-9a-f]{64}$'),
    normalized_context_json JSONB NOT NULL,
    intent_key TEXT NOT NULL CHECK (intent_key <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_account_id, signature_hash)
);

CREATE INDEX idx_action_signatures_seller_type ON action_signatures (seller_account_id, action_type);

CREATE TABLE action_signature_state (
    signature_id BIGINT PRIMARY KEY REFERENCES action_signatures(id) ON DELETE RESTRICT,
    active_case_id UUID NULL,
    suppressed_until TIMESTAMPTZ NULL,
    suppression_source TEXT NULL,
    suppression_reason TEXT NULL,
    last_occurrence_no INTEGER NOT NULL DEFAULT 0 CHECK (last_occurrence_no >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE action_cases (
    id UUID PRIMARY KEY,
    signature_id BIGINT NOT NULL REFERENCES action_signatures(id) ON DELETE RESTRICT,
    occurrence_no INTEGER NOT NULL CHECK (occurrence_no >= 1),
    opened_by_publication_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ NULL,
    UNIQUE (signature_id, occurrence_no)
);

ALTER TABLE action_signature_state
    ADD CONSTRAINT fk_action_signature_state_active_case
    FOREIGN KEY (active_case_id) REFERENCES action_cases(id) ON DELETE RESTRICT;

CREATE TABLE action_case_state (
    case_id UUID PRIMARY KEY REFERENCES action_cases(id) ON DELETE RESTRICT,
    system_state TEXT NOT NULL CHECK (system_state IN ('ACTIVE','SUSPENDED','RESOLVED','SUPERSEDED')),
    user_state TEXT NOT NULL CHECK (user_state IN ('OPEN','DONE','DISMISSED')),
    current_proposal_id UUID NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE action_case_proposals (
    id UUID PRIMARY KEY,
    case_id UUID NOT NULL REFERENCES action_cases(id) ON DELETE RESTRICT,
    publication_id BIGINT NOT NULL,
    recommendation_id TEXT NOT NULL,
    action_type TEXT NOT NULL,
    parameters_json JSONB NOT NULL,
    display_order INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_action_case_proposals_pub_case ON action_case_proposals (publication_id, case_id);

ALTER TABLE action_case_state
    ADD CONSTRAINT fk_action_case_state_current_proposal
    FOREIGN KEY (current_proposal_id) REFERENCES action_case_proposals(id) ON DELETE RESTRICT;

CREATE TABLE action_proposal_supports (
    id BIGSERIAL PRIMARY KEY,
    proposal_id UUID NOT NULL REFERENCES action_case_proposals(id) ON DELETE RESTRICT,
    support_key TEXT NOT NULL CHECK (support_key <> ''),
    support_role TEXT NOT NULL CHECK (support_role IN ('REQUIRED_SUPPORT','CONTEXT','COUNTEREVIDENCE')),
    claim TEXT NOT NULL,
    fact_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    finding_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    UNIQUE (proposal_id, support_key)
);

CREATE TABLE action_case_assessments (
    id UUID PRIMARY KEY,
    previous_proposal_id UUID NOT NULL REFERENCES action_case_proposals(id) ON DELETE RESTRICT,
    analysis_run_id BIGINT NOT NULL REFERENCES analysis_runs(id) ON DELETE RESTRICT,
    new_publication_id BIGINT NOT NULL,
    outcome TEXT NOT NULL CHECK (outcome IN ('FULLY_SUPPORTED','PARTIALLY_SUPPORTED','NOT_SUPPORTED','UNVERIFIABLE','SUPERSEDED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (analysis_run_id, previous_proposal_id)
);

CREATE TABLE action_support_assessments (
    id BIGSERIAL PRIMARY KEY,
    assessment_id UUID NOT NULL REFERENCES action_case_assessments(id) ON DELETE RESTRICT,
    support_key TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('HOLDS','CHANGED','NOT_VERIFIABLE')),
    current_fact_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    UNIQUE (assessment_id, support_key)
);

CREATE TABLE action_user_events (
    id UUID PRIMARY KEY,
    case_id UUID NOT NULL REFERENCES action_cases(id) ON DELETE RESTRICT,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE RESTRICT,
    user_id BIGINT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('DONE','DISMISSED','REOPENED')),
    reason_code TEXT NULL,
    comment TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_action_user_events_case ON action_user_events (case_id, created_at);

CREATE TABLE action_system_events (
    id UUID PRIMARY KEY,
    case_id UUID NOT NULL REFERENCES action_cases(id) ON DELETE RESTRICT,
    signature_id BIGINT NOT NULL REFERENCES action_signatures(id) ON DELETE RESTRICT,
    event_type TEXT NOT NULL CHECK (event_type IN ('SUSPENDED','RESOLVED','SUPERSEDED','RECURRED','REAFFIRMED','BASIS_CHANGED')),
    publication_id BIGINT NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_action_system_events_case ON action_system_events (case_id, created_at);
