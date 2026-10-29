-- Stage 5: Unified Analysis Publication.
-- Introduces rollout controls, the publication ledger, the unified "current" pointer
-- (separate from the legacy seller_current_ai_snapshots pointer), the action mapping
-- between unified analysis actions and existing action_items rows, and an audit trail
-- for publish/promote/rollback/set_mode operations.

-- 1. Per-seller rollout controls. Missing rows are treated as legacy in code, so this
--    table is intentionally left empty by this migration (no backfill for all sellers).
CREATE TABLE IF NOT EXISTS seller_analysis_settings (
    seller_account_id BIGINT PRIMARY KEY REFERENCES seller_accounts(id) ON DELETE CASCADE,
    analysis_mode TEXT NOT NULL DEFAULT 'legacy',
    auto_run_enabled BOOLEAN NOT NULL DEFAULT false,
    auto_publish_enabled BOOLEAN NOT NULL DEFAULT false,
    legacy_fallback_enabled BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by TEXT NOT NULL DEFAULT 'system',
    CONSTRAINT chk_seller_analysis_settings_mode CHECK (
        analysis_mode IN ('legacy', 'shadow', 'unified')
    )
);

-- 2. Publication ledger: one row per attempt to publish (or shadow-project) a unified
--    analysis run's outputs, tracking provenance hashes and downstream projection links.
CREATE TABLE IF NOT EXISTS unified_analysis_publications (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id),
    metric_date DATE NOT NULL,
    run_id BIGINT NOT NULL REFERENCES ai_pipeline_runs(id),

    pack_id TEXT NOT NULL,
    pack_hash TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    prompt_hash TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    schema_hash TEXT NOT NULL,
    output_hash TEXT NOT NULL,
    adapter_version TEXT NOT NULL,
    adapter_hash TEXT NOT NULL,

    mode TEXT NOT NULL,
    visibility TEXT NOT NULL,
    status TEXT NOT NULL,
    analysis_outcome TEXT NOT NULL,

    dashboard_run_id UUID NULL,
    actions_run_id BIGINT NULL,
    dashboard_data_snapshot_id BIGINT NULL,

    dashboard_record_count INT NOT NULL DEFAULT 0,
    actions_record_count INT NOT NULL DEFAULT 0,
    top_actions_count INT NOT NULL DEFAULT 0,
    recommended_actions_count INT NOT NULL DEFAULT 0,

    dashboard_projection_hash TEXT NOT NULL DEFAULT '',
    actions_projection_hash TEXT NOT NULL DEFAULT '',
    bundle_hash TEXT NOT NULL DEFAULT '',

    published_at TIMESTAMPTZ NULL,
    superseded_at TIMESTAMPTZ NULL,
    superseded_by_publication_id BIGINT NULL REFERENCES unified_analysis_publications(id),

    error_code TEXT NULL,
    error_message TEXT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_unified_analysis_publications_run_id UNIQUE (run_id),
    CONSTRAINT chk_unified_analysis_publications_mode CHECK (
        mode IN ('shadow', 'unified')
    ),
    CONSTRAINT chk_unified_analysis_publications_visibility CHECK (
        visibility IN ('shadow', 'visible')
    ),
    CONSTRAINT chk_unified_analysis_publications_status CHECK (
        status IN (
            'created',
            'projecting',
            'ready',
            'publishing',
            'published',
            'publication_failed',
            'rolled_back',
            'superseded'
        )
    )
);

CREATE INDEX IF NOT EXISTS idx_unified_analysis_publications_seller_metric_date
    ON unified_analysis_publications (seller_account_id, metric_date);

CREATE INDEX IF NOT EXISTS idx_unified_analysis_publications_seller_status
    ON unified_analysis_publications (seller_account_id, status);

CREATE INDEX IF NOT EXISTS idx_unified_analysis_publications_seller_visibility_status
    ON unified_analysis_publications (seller_account_id, visibility, status);

-- 3. Unified "current" visible pointer. Kept separate from the legacy
--    seller_current_ai_snapshots pointer so legacy and unified rollout can coexist.
CREATE TABLE IF NOT EXISTS seller_current_unified_publications (
    seller_account_id BIGINT PRIMARY KEY REFERENCES seller_accounts(id) ON DELETE CASCADE,
    publication_id BIGINT NOT NULL REFERENCES unified_analysis_publications(id),
    metric_date DATE NOT NULL,
    source_mode TEXT NOT NULL DEFAULT 'unified',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_seller_current_unified_publications_source_mode CHECK (
        source_mode IN ('unified')
    )
);

-- 4. Mapping between unified analysis action ids and materialized action_items rows
--    for a given publication (used to reconstruct top/recommended action ordering).
CREATE TABLE IF NOT EXISTS unified_publication_action_map (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES unified_analysis_publications(id) ON DELETE CASCADE,
    analysis_action_id TEXT NOT NULL,
    action_item_id BIGINT NOT NULL REFERENCES action_items(id),
    is_top BOOLEAN NOT NULL DEFAULT false,
    is_recommended BOOLEAN NOT NULL DEFAULT false,
    sort_order INT NOT NULL DEFAULT 0,

    CONSTRAINT uq_unified_publication_action_map_action_id UNIQUE (publication_id, analysis_action_id),
    CONSTRAINT uq_unified_publication_action_map_item_id UNIQUE (publication_id, action_item_id)
);

CREATE INDEX IF NOT EXISTS idx_unified_publication_action_map_publication_top
    ON unified_publication_action_map (publication_id, is_top);

-- 5. Audit trail for publish/promote/rollback/set_mode operations.
CREATE TABLE IF NOT EXISTS unified_analysis_publication_audit (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL,
    operation TEXT NOT NULL,
    actor TEXT NOT NULL,
    run_id BIGINT NULL,
    publication_id BIGINT NULL,
    previous_publication_id BIGINT NULL,
    new_publication_id BIGINT NULL,
    reason TEXT NULL,
    payload_json JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_unified_analysis_publication_audit_seller_created
    ON unified_analysis_publication_audit (seller_account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_unified_analysis_publication_audit_publication
    ON unified_analysis_publication_audit (publication_id);

-- 6. Link ai_pipeline_runs to the publication it produced (nullable; legacy runs
--    and runs that never reach a publication decision leave this NULL).
ALTER TABLE ai_pipeline_runs
    ADD COLUMN IF NOT EXISTS unified_publication_id BIGINT NULL
        REFERENCES unified_analysis_publications(id);

CREATE INDEX IF NOT EXISTS idx_ai_pipeline_runs_unified_publication_id
    ON ai_pipeline_runs (unified_publication_id)
    WHERE unified_publication_id IS NOT NULL;

-- 7. publication_failed is a new terminal ai_pipeline_runs.status value for Stage 5
--    (publishing/published were already declared as "reserved" by migration 000039).
--    Recreate the unique index to also exclude publishing/published/publication_failed:
--    a unified_analysis run that has reached (or definitively failed) publication must
--    never block creating a new run for the same seller+metric_date.
ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS chk_ai_pipeline_runs_status;

ALTER TABLE ai_pipeline_runs
    ADD CONSTRAINT chk_ai_pipeline_runs_status CHECK (
        status IN (
            -- legacy seller AI pipeline
            'running',
            'completed',
            'completed_partial',
            'failed',
            'context_failed',
            'dashboard_failed',
            'actions_failed',
            'empty',
            -- unified analysis provider (stage 3)
            'created',
            'pack_building',
            'pack_validating',
            'pack_ready',
            'provider_container_creating',
            'provider_container_ready',
            'provider_file_uploading',
            'provider_file_ready',
            'provider_response_starting',
            'provider_running',
            'provider_completed',
            'cleanup_pending',
            'cleanup_running',
            'cleanup_completed',
            'cancelled',
            -- unified analysis output validation (stage 4) / publication (stage 5)
            'output_validating',
            'publication_ready',
            'publishing',
            'published',
            'publication_blocked',
            'publication_failed'
        )
    );

DROP INDEX IF EXISTS uq_ai_pipeline_runs_one_active_unified_analysis;

CREATE UNIQUE INDEX uq_ai_pipeline_runs_one_active_unified_analysis
    ON ai_pipeline_runs (seller_account_id, metric_date)
    WHERE mode = 'unified_analysis'
      AND status NOT IN (
          'failed',
          'cancelled',
          'provider_completed',
          'publication_ready',
          'publication_blocked',
          'publishing',
          'published',
          'publication_failed',
          'cleanup_completed'
      );
