-- Prompt №13: immutable Publications, projections, current pointer, outbox.

CREATE TABLE analysis_publications (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE RESTRICT,
    publication_generation BIGINT NOT NULL CHECK (publication_generation >= 1),
    analysis_run_id BIGINT NOT NULL REFERENCES analysis_runs(id) ON DELETE RESTRICT,
    analysis_date DATE NOT NULL,
    analysis_cutoff_at TIMESTAMPTZ NOT NULL,
    factual_universe_id UUID NOT NULL,
    factual_universe_hash TEXT NOT NULL CHECK (factual_universe_hash ~ '^[0-9a-f]{64}$'),
    contract_bundle_hash TEXT NOT NULL,
    publication_content_hash TEXT NOT NULL CHECK (publication_content_hash ~ '^[0-9a-f]{64}$'),
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (analysis_run_id),
    UNIQUE (seller_account_id, publication_generation),
    UNIQUE (id, seller_account_id)
);

CREATE TABLE seller_current_publications (
    seller_account_id BIGINT PRIMARY KEY REFERENCES seller_accounts(id) ON DELETE RESTRICT,
    publication_id BIGINT NOT NULL REFERENCES analysis_publications(id) ON DELETE RESTRICT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE publication_release_events (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES analysis_publications(id) ON DELETE RESTRICT,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE RESTRICT,
    event_type TEXT NOT NULL CHECK (event_type IN ('PUBLISHED','MADE_CURRENT','WITHDRAWN','ROLLBACK_SELECTED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE publication_components (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES analysis_publications(id) ON DELETE RESTRICT,
    component_type TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    artifact_id BIGINT NULL REFERENCES analysis_artifacts(id) ON DELETE RESTRICT,
    sha256 TEXT NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    UNIQUE (publication_id, component_type)
);

CREATE TABLE publication_findings (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES analysis_publications(id) ON DELETE RESTRICT,
    finding_ref TEXT NOT NULL,
    ordinal INTEGER NOT NULL,
    primary_kind TEXT NOT NULL,
    primary_id TEXT NOT NULL,
    statement TEXT NOT NULL,
    display_order INTEGER NOT NULL,
    grounding_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    UNIQUE (publication_id, finding_ref)
);

CREATE TABLE publication_recommendations (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES analysis_publications(id) ON DELETE RESTRICT,
    recommendation_ref TEXT NOT NULL,
    ordinal INTEGER NOT NULL,
    actionability TEXT NOT NULL,
    summary TEXT NOT NULL,
    finding_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    display_order INTEGER NOT NULL,
    UNIQUE (publication_id, recommendation_ref)
);

CREATE TABLE publication_projections (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES analysis_publications(id) ON DELETE RESTRICT,
    projection_key TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    payload_json JSONB NOT NULL,
    payload_hash TEXT NOT NULL CHECK (payload_hash ~ '^[0-9a-f]{64}$'),
    UNIQUE (publication_id, projection_key)
);

CREATE TABLE publication_stock_sku_rows (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES analysis_publications(id) ON DELETE RESTRICT,
    sku_id TEXT NOT NULL,
    row_json JSONB NOT NULL,
    UNIQUE (publication_id, sku_id)
);

CREATE TABLE publication_ad_campaign_rows (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES analysis_publications(id) ON DELETE RESTRICT,
    campaign_id TEXT NOT NULL,
    row_json JSONB NOT NULL,
    UNIQUE (publication_id, campaign_id)
);

CREATE TABLE publication_sku_focus_rows (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES analysis_publications(id) ON DELETE RESTRICT,
    sku_id TEXT NOT NULL,
    row_json JSONB NOT NULL,
    UNIQUE (publication_id, sku_id)
);

CREATE TABLE publication_action_groups (
    id UUID PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES analysis_publications(id) ON DELETE RESTRICT,
    recommendation_id TEXT NOT NULL,
    action_type TEXT NOT NULL,
    target_kind TEXT NOT NULL,
    family TEXT NOT NULL,
    display_order INTEGER NOT NULL,
    display_title TEXT NOT NULL,
    display_summary TEXT NOT NULL,
    presentation_template_key TEXT NOT NULL,
    UNIQUE (publication_id, recommendation_id, action_type, target_kind)
);

CREATE TABLE publication_action_group_members (
    id BIGSERIAL PRIMARY KEY,
    group_id UUID NOT NULL REFERENCES publication_action_groups(id) ON DELETE RESTRICT,
    case_id UUID NOT NULL REFERENCES action_cases(id) ON DELETE RESTRICT,
    proposal_id UUID NOT NULL REFERENCES action_case_proposals(id) ON DELETE RESTRICT,
    member_order INTEGER NOT NULL,
    UNIQUE (group_id, proposal_id)
);

CREATE TABLE analysis_outbox_events (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    payload_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NULL
);

CREATE INDEX idx_analysis_outbox_unprocessed ON analysis_outbox_events (created_at)
    WHERE processed_at IS NULL;

ALTER TABLE action_cases
    ADD CONSTRAINT fk_action_cases_opened_by_publication
    FOREIGN KEY (opened_by_publication_id) REFERENCES analysis_publications(id) ON DELETE RESTRICT;

ALTER TABLE action_case_proposals
    ADD CONSTRAINT fk_action_case_proposals_publication
    FOREIGN KEY (publication_id) REFERENCES analysis_publications(id) ON DELETE RESTRICT;

ALTER TABLE action_case_assessments
    ADD CONSTRAINT fk_action_case_assessments_publication
    FOREIGN KEY (new_publication_id) REFERENCES analysis_publications(id) ON DELETE RESTRICT;

ALTER TABLE analysis_runs
    ADD CONSTRAINT fk_analysis_runs_base_publication
    FOREIGN KEY (base_publication_id) REFERENCES analysis_publications(id) ON DELETE RESTRICT;

CREATE OR REPLACE FUNCTION forbid_publication_content_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION '% is immutable publication content', TG_TABLE_NAME USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER trg_analysis_publications_immutable
    BEFORE UPDATE OR DELETE ON analysis_publications
    FOR EACH ROW EXECUTE FUNCTION forbid_publication_content_mutation();
CREATE TRIGGER trg_publication_components_immutable
    BEFORE UPDATE OR DELETE ON publication_components
    FOR EACH ROW EXECUTE FUNCTION forbid_publication_content_mutation();
CREATE TRIGGER trg_publication_findings_immutable
    BEFORE UPDATE OR DELETE ON publication_findings
    FOR EACH ROW EXECUTE FUNCTION forbid_publication_content_mutation();
CREATE TRIGGER trg_publication_recommendations_immutable
    BEFORE UPDATE OR DELETE ON publication_recommendations
    FOR EACH ROW EXECUTE FUNCTION forbid_publication_content_mutation();
CREATE TRIGGER trg_publication_projections_immutable
    BEFORE UPDATE OR DELETE ON publication_projections
    FOR EACH ROW EXECUTE FUNCTION forbid_publication_content_mutation();
CREATE TRIGGER trg_publication_stock_sku_rows_immutable
    BEFORE UPDATE OR DELETE ON publication_stock_sku_rows
    FOR EACH ROW EXECUTE FUNCTION forbid_publication_content_mutation();
CREATE TRIGGER trg_publication_ad_campaign_rows_immutable
    BEFORE UPDATE OR DELETE ON publication_ad_campaign_rows
    FOR EACH ROW EXECUTE FUNCTION forbid_publication_content_mutation();
CREATE TRIGGER trg_publication_sku_focus_rows_immutable
    BEFORE UPDATE OR DELETE ON publication_sku_focus_rows
    FOR EACH ROW EXECUTE FUNCTION forbid_publication_content_mutation();
CREATE TRIGGER trg_publication_action_groups_immutable
    BEFORE UPDATE OR DELETE ON publication_action_groups
    FOR EACH ROW EXECUTE FUNCTION forbid_publication_content_mutation();
CREATE TRIGGER trg_publication_action_group_members_immutable
    BEFORE UPDATE OR DELETE ON publication_action_group_members
    FOR EACH ROW EXECUTE FUNCTION forbid_publication_content_mutation();
CREATE TRIGGER trg_publication_release_events_append_only
    BEFORE UPDATE OR DELETE ON publication_release_events
    FOR EACH ROW EXECUTE FUNCTION forbid_analysis_append_only_mutation();
