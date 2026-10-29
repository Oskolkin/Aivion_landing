ALTER TABLE ad_campaign_skus
    DROP COLUMN IF EXISTS product_title,
    DROP COLUMN IF EXISTS bid_raw_micro,
    DROP COLUMN IF EXISTS bid_amount,
    DROP COLUMN IF EXISTS bid_unit_assumption,
    DROP COLUMN IF EXISTS captured_at;

ALTER TABLE ad_metrics_daily
    DROP COLUMN IF EXISTS campaign_name;

ALTER TABLE ad_campaigns
    DROP COLUMN IF EXISTS payment_type,
    DROP COLUMN IF EXISTS placement_json,
    DROP COLUMN IF EXISTS budget_raw_micro,
    DROP COLUMN IF EXISTS daily_budget_raw_micro,
    DROP COLUMN IF EXISTS weekly_budget_raw_micro,
    DROP COLUMN IF EXISTS budget_weekly,
    DROP COLUMN IF EXISTS from_date,
    DROP COLUMN IF EXISTS to_date,
    DROP COLUMN IF EXISTS autostop_status,
    DROP COLUMN IF EXISTS is_autocreated,
    DROP COLUMN IF EXISTS product_autopilot_strategy,
    DROP COLUMN IF EXISTS product_campaign_mode,
    DROP COLUMN IF EXISTS performance_created_at,
    DROP COLUMN IF EXISTS performance_updated_at,
    DROP COLUMN IF EXISTS captured_at;

DROP INDEX IF EXISTS uq_stock_analytics_snapshots_seller_date_sku_wh_cluster;

ALTER TABLE stock_analytics_snapshots
    DROP COLUMN IF EXISTS cluster_id,
    DROP COLUMN IF EXISTS cluster_name,
    DROP COLUMN IF EXISTS macrolocal_cluster_id,
    DROP COLUMN IF EXISTS ads_cluster,
    DROP COLUMN IF EXISTS idc_cluster,
    DROP COLUMN IF EXISTS days_without_sales_cluster,
    DROP COLUMN IF EXISTS expiring_stock_count,
    DROP COLUMN IF EXISTS other_stock_count,
    DROP COLUMN IF EXISTS stock_defect_stock_count,
    DROP COLUMN IF EXISTS transit_defect_stock_count,
    DROP COLUMN IF EXISTS item_tags_json,
    DROP COLUMN IF EXISTS captured_at,
    DROP COLUMN IF EXISTS raw_response_json;

CREATE UNIQUE INDEX uq_stock_analytics_snapshots_seller_date_sku_wh
    ON stock_analytics_snapshots(seller_account_id, snapshot_date, sku, warehouse_id);

DROP TABLE IF EXISTS ad_campaign_objects;
DROP TABLE IF EXISTS ozon_enrichment_sync_runs;
DROP TABLE IF EXISTS ozon_related_sku_errors;
DROP TABLE IF EXISTS ozon_related_skus;
DROP TABLE IF EXISTS ozon_product_info_snapshots;
DROP TABLE IF EXISTS ozon_api_capabilities;
