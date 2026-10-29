-- DEV / LOCAL ONLY.
-- Verification queries for step 14 initial-sync end-to-end checks.

-- 1) Core auth/account/connection entities.
SELECT id, email, status, created_at, updated_at
FROM users
ORDER BY id;

SELECT id, user_id, name, status, created_at, updated_at
FROM seller_accounts
ORDER BY id;

SELECT id, user_id, expires_at, created_at, revoked_at
FROM sessions
ORDER BY id;

SELECT id, seller_account_id, status, last_check_at, last_check_result, last_error, created_at, updated_at
FROM ozon_connections
ORDER BY id;

-- 2) Orchestration/internal ingestion tracking.
SELECT id, seller_account_id, type, status, started_at, finished_at, error_message, created_at
FROM sync_jobs
ORDER BY id DESC;

SELECT id, seller_account_id, sync_job_id, domain, status, source_cursor, records_received, records_imported, records_failed, started_at, finished_at, error_message, created_at
FROM import_jobs
ORDER BY id DESC;

SELECT id, seller_account_id, domain, cursor_type, cursor_value, updated_at
FROM sync_cursors
ORDER BY seller_account_id, domain, cursor_type;

SELECT id, seller_account_id, import_job_id, domain, source, request_key, storage_bucket, storage_object_key, payload_hash, received_at
FROM raw_payloads
ORDER BY id DESC;

-- 3) Domain entities after ingestion.
SELECT id, seller_account_id, ozon_product_id, offer_id, sku, name, status, is_archived, source_updated_at, created_at, updated_at
FROM products
ORDER BY id DESC;

SELECT id, seller_account_id, ozon_order_id, posting_number, status, created_at_source, processed_at_source, total_amount, currency_code, created_at, updated_at
FROM orders
ORDER BY id DESC;

SELECT id, seller_account_id, ozon_sale_id, ozon_order_id, posting_number, quantity, amount, currency_code, sale_date, created_at, updated_at
FROM sales
ORDER BY id DESC;

SELECT id, seller_account_id, product_external_id, warehouse_external_id, quantity_total, quantity_reserved, quantity_available, snapshot_at, created_at, updated_at
FROM stocks
ORDER BY id DESC;

-- 4) Quick row counters by table.
SELECT 'users' AS table_name, COUNT(*) AS row_count FROM users
UNION ALL SELECT 'seller_accounts', COUNT(*) FROM seller_accounts
UNION ALL SELECT 'sessions', COUNT(*) FROM sessions
UNION ALL SELECT 'ozon_connections', COUNT(*) FROM ozon_connections
UNION ALL SELECT 'sync_jobs', COUNT(*) FROM sync_jobs
UNION ALL SELECT 'import_jobs', COUNT(*) FROM import_jobs
UNION ALL SELECT 'sync_cursors', COUNT(*) FROM sync_cursors
UNION ALL SELECT 'raw_payloads', COUNT(*) FROM raw_payloads
UNION ALL SELECT 'products', COUNT(*) FROM products
UNION ALL SELECT 'orders', COUNT(*) FROM orders
UNION ALL SELECT 'sales', COUNT(*) FROM sales
UNION ALL SELECT 'stocks', COUNT(*) FROM stocks
ORDER BY table_name;

-- 5) Duplicate checks for key domain uniqueness constraints.
-- Expected result for each query: 0 rows.
SELECT seller_account_id, ozon_product_id, COUNT(*) AS duplicate_count
FROM products
GROUP BY seller_account_id, ozon_product_id
HAVING COUNT(*) > 1;

SELECT seller_account_id, ozon_order_id, COUNT(*) AS duplicate_count
FROM orders
GROUP BY seller_account_id, ozon_order_id
HAVING COUNT(*) > 1;

SELECT seller_account_id, ozon_sale_id, COUNT(*) AS duplicate_count
FROM sales
GROUP BY seller_account_id, ozon_sale_id
HAVING COUNT(*) > 1;

SELECT seller_account_id, product_external_id, warehouse_external_id, COUNT(*) AS duplicate_count
FROM stocks
GROUP BY seller_account_id, product_external_id, warehouse_external_id
HAVING COUNT(*) > 1;
