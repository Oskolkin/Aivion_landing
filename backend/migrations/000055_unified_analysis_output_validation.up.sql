-- Stage 4: treat publication_* as inactive for the one-active-run unique index,
-- and include output_validating / provider_completed analysis runs in recovery scans.

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
