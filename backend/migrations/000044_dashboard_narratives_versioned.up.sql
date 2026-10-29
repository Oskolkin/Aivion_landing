ALTER TABLE ai_dashboard_narratives
    DROP CONSTRAINT IF EXISTS uq_ai_dashboard_narratives_seller_date;

CREATE INDEX IF NOT EXISTS idx_ai_dashboard_narratives_seller_metric_created
    ON ai_dashboard_narratives (seller_account_id, metric_date DESC, created_at DESC);
