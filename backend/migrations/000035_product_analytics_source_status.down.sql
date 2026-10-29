ALTER TABLE product_analytics_daily
    DROP COLUMN IF EXISTS source_status,
    DROP COLUMN IF EXISTS source_message;
