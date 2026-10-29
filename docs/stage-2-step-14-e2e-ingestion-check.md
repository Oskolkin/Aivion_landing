# Stage 2 - Step 14: End-to-End Ingestion Check (Local Dev)

This guide provides a reproducible local verification flow for the ingestion contour "from zero state" without adding new ingestion business logic.

## Scope

Checks covered in this scenario:

1. Full dev DB cleanup.
2. Run backend API, worker, and frontend.
3. Register a new user and authenticate.
4. Connect Ozon Seller API credentials.
5. Run connection check.
6. Start initial sync.
7. Validate data in `sync_jobs`, `import_jobs`, `raw_payloads`, `sync_cursors`, `products`, `orders`, `sales`, `stocks`.
8. Validate backend endpoint `GET /api/v1/integrations/ozon/status`.
9. Validate frontend page `/app/sync-status`.

## Prerequisites

- Infrastructure is running (Postgres, Redis, MinIO): `make infra-up`
- Backend env file exists: `backend/.env.local`
- Frontend env file exists: `frontend/.env.local`
- `psql` and `curl` are available in your shell

Recommended `backend/.env.local` and `frontend/.env.local` values are aligned with root `.env.example`, including:

- `BACKEND_PORT=8081`
- `DATABASE_URL=postgres://postgres:postgres@localhost:55432/marketplace_ai?sslmode=disable`
- `NEXT_PUBLIC_API_BASE_URL=http://localhost:8081`

## 1) Full dev DB cleanup

Run from repository root:

```bash
make dev-db-reset
```

Equivalent direct command:

```bash
psql "postgres://postgres:postgres@localhost:55432/marketplace_ai?sslmode=disable" -f scripts/dev/sql/reset_dev_data.sql
```

Notes:

- Script file: `scripts/dev/sql/reset_dev_data.sql`
- Uses `TRUNCATE ... RESTART IDENTITY CASCADE`
- Does not touch `schema_migrations`
- Intended only for local/dev databases

## 2) Start backend API

In terminal #1:

```bash
cd backend
go run ./cmd/api
```

Backend listens on port from config (`BACKEND_PORT`), default is `8081` in this project local setup.

## 3) Start worker

In terminal #2:

```bash
cd backend
go run ./cmd/worker
```

Worker processes Asynq ingestion jobs triggered by initial sync.

## 4) Start frontend

In terminal #3:

```bash
cd frontend
npm run dev
```

Frontend runs on `http://localhost:3000` by default.

## 5) Register user and create authenticated session

Use a cookie jar so subsequent requests stay authenticated.

```bash
curl -i -c cookies.txt -X POST "http://localhost:8081/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "step14-e2e@example.com",
    "password": "StrongPass123!",
    "password_confirm": "StrongPass123!"
  }'
```

Validate session with:

```bash
curl -i -b cookies.txt "http://localhost:8081/api/v1/auth/me"
```

Expected: `200 OK` with `user` and `seller_account`.

## 6) Connect Ozon Seller API

Create connection:

```bash
curl -i -b cookies.txt -X POST "http://localhost:8081/api/v1/integrations/ozon/" \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "YOUR_OZON_CLIENT_ID",
    "api_key": "YOUR_OZON_API_KEY"
  }'
```

Read current connection:

```bash
curl -i -b cookies.txt "http://localhost:8081/api/v1/integrations/ozon/"
```

## 7) Run connection check

```bash
curl -i -b cookies.txt -X POST "http://localhost:8081/api/v1/integrations/ozon/check"
```

Expected: `200 OK` and response with `status`, `checked_at`, `message`, `error_code`.

## 8) Start initial sync

```bash
curl -i -b cookies.txt -X POST "http://localhost:8081/api/v1/integrations/ozon/initial-sync"
```

Expected: `202 Accepted` and `sync_job` payload.

Because ingestion is async, wait for worker to process tasks and re-check status endpoint.

## 9) Check backend ingestion status endpoint

```bash
curl -i -b cookies.txt "http://localhost:8081/api/v1/integrations/ozon/status"
```

Expected structure includes:

- `connection_status`
- `current_sync`
- `last_successful_sync_at`
- `latest_import_jobs`

For successful initial sync, `current_sync.status` should eventually become `completed`, and import jobs should be present for configured domains.

## 10) Check frontend sync-status page

Open in browser (authenticated session in frontend UI):

- `http://localhost:3000/app/sync-status`

Verify that page displays:

- connection status and latest check fields;
- sync summary (`current_sync`, timestamps, error);
- import jobs list with domains and counters (`records_received`, `records_imported`, `records_failed`).

## 11) Validate DB state with SQL checks

Run:

```bash
make dev-db-check-initial-sync
```

Equivalent direct command:

```bash
psql "postgres://postgres:postgres@localhost:55432/marketplace_ai?sslmode=disable" -f scripts/dev/sql/check_initial_sync.sql
```

The script contains ready-to-use `SELECT` checks for:

- `users`
- `seller_accounts`
- `sessions`
- `ozon_connections`
- `sync_jobs`
- `import_jobs`
- `sync_cursors`
- `raw_payloads`
- `products`
- `orders`
- `sales`
- `stocks`

And duplicate checks (expected: zero rows returned):

- products: (`seller_account_id`, `ozon_product_id`)
- orders: (`seller_account_id`, `ozon_order_id`)
- sales: (`seller_account_id`, `ozon_sale_id`)
- stocks: (`seller_account_id`, `product_external_id`, `warehouse_external_id`)

## Success criteria for step 14

Step 14 e2e check can be considered successful when all are true:

1. Auth and seller account are created from a clean DB.
2. Ozon connection is saved and connection check returns success response.
3. Initial sync starts and reaches terminal successful state in status endpoint.
4. `sync_jobs`, `import_jobs`, `raw_payloads`, `sync_cursors`, and domain tables are populated consistently.
5. Duplicate-check queries return zero rows.
6. Backend `GET /api/v1/integrations/ozon/status` and frontend `/app/sync-status` reflect the same ingestion progress/result.
