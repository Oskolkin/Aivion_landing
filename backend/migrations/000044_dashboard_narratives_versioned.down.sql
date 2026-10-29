DROP INDEX IF EXISTS idx_ai_dashboard_narratives_seller_metric_created;

DELETE FROM ai_dashboard_narratives d
WHERE d.id IN (
    SELECT id
    FROM (
        SELECT
            id,
            ROW_NUMBER() OVER (
                PARTITION BY seller_account_id, metric_date
                ORDER BY created_at DESC, id DESC
            ) AS rn
        FROM ai_dashboard_narratives
    ) ranked
    WHERE ranked.rn > 1
);

ALTER TABLE ai_dashboard_narratives
    ADD CONSTRAINT uq_ai_dashboard_narratives_seller_date UNIQUE (seller_account_id, metric_date);
