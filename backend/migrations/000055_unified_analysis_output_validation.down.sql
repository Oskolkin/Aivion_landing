-- Restore Stage 3 unique index definition.

DROP INDEX IF EXISTS uq_ai_pipeline_runs_one_active_unified_analysis;

CREATE UNIQUE INDEX uq_ai_pipeline_runs_one_active_unified_analysis
    ON ai_pipeline_runs (seller_account_id, metric_date)
    WHERE mode = 'unified_analysis'
      AND status NOT IN ('failed', 'cancelled', 'provider_completed', 'cleanup_completed');
