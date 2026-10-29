DROP INDEX IF EXISTS idx_alert_runs_seller_type_started_desc;
DROP INDEX IF EXISTS idx_alert_runs_seller_status;
DROP INDEX IF EXISTS idx_alert_runs_seller_started_desc;
DROP TABLE IF EXISTS alert_runs;

DROP INDEX IF EXISTS idx_alerts_evidence_payload_gin;
DROP INDEX IF EXISTS idx_alerts_seller_last_seen_desc;
DROP INDEX IF EXISTS idx_alerts_seller_entity_sku;
DROP INDEX IF EXISTS idx_alerts_seller_entity_type;
DROP INDEX IF EXISTS idx_alerts_seller_severity_status;
DROP INDEX IF EXISTS idx_alerts_seller_group_status;
DROP INDEX IF EXISTS idx_alerts_seller_status;
DROP TABLE IF EXISTS alerts;
