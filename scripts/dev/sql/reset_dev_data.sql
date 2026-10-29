-- DEV / LOCAL ONLY.
-- This script irreversibly removes application data for local end-to-end checks.
-- Do NOT run on staging/production databases.

BEGIN;

TRUNCATE TABLE
    sessions,
    ozon_connections,
    sync_cursors,
    raw_payloads,
    import_jobs,
    sync_jobs,
    products,
    orders,
    sales,
    stocks,
    seller_accounts,
    users
RESTART IDENTITY CASCADE;

COMMIT;
