ALTER TABLE daily_account_metrics
    ADD COLUMN IF NOT EXISTS units_sold INTEGER;

ALTER TABLE daily_sku_metrics
    ADD COLUMN IF NOT EXISTS units_sold INTEGER;
