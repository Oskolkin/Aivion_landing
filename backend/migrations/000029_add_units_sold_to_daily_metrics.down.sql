ALTER TABLE daily_sku_metrics
    DROP COLUMN IF EXISTS units_sold;

ALTER TABLE daily_account_metrics
    DROP COLUMN IF EXISTS units_sold;
