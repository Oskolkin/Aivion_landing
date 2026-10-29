ALTER TABLE product_analytics_daily
    ADD COLUMN IF NOT EXISTS source_status TEXT,
    ADD COLUMN IF NOT EXISTS source_message TEXT;
