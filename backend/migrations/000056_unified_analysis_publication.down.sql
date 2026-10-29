-- Reverse Stage 5: Unified Analysis Publication.

-- 7. Restore the Stage 4 unique index and Stage 3 status check (drop publication_failed).
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
          'cleanup_completed'
      );

ALTER TABLE ai_pipeline_runs
    DROP CONSTRAINT IF EXISTS chk_ai_pipeline_runs_status;

ALTER TABLE ai_pipeline_runs
    ADD CONSTRAINT chk_ai_pipeline_runs_status CHECK (
        status IN (
            'running',
            'completed',
            'completed_partial',
            'failed',
            'context_failed',
            'dashboard_failed',
            'actions_failed',
            'empty',
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
            'output_validating',
            'publication_ready',
            'publishing',
            'published',
            'publication_blocked'
        )
    );

DROP INDEX IF EXISTS idx_ai_pipeline_runs_unified_publication_id;

ALTER TABLE ai_pipeline_runs
    DROP COLUMN IF EXISTS unified_publication_id;

DROP INDEX IF EXISTS idx_unified_analysis_publication_audit_publication;
DROP INDEX IF EXISTS idx_unified_analysis_publication_audit_seller_created;
DROP TABLE IF EXISTS unified_analysis_publication_audit;

DROP INDEX IF EXISTS idx_unified_publication_action_map_publication_top;
DROP TABLE IF EXISTS unified_publication_action_map;

DROP TABLE IF EXISTS seller_current_unified_publications;

DROP INDEX IF EXISTS idx_unified_analysis_publications_seller_visibility_status;
DROP INDEX IF EXISTS idx_unified_analysis_publications_seller_status;
DROP INDEX IF EXISTS idx_unified_analysis_publications_seller_metric_date;
DROP TABLE IF EXISTS unified_analysis_publications;

DROP TABLE IF EXISTS seller_analysis_settings;
