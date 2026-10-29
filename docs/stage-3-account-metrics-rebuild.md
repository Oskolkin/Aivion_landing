# Stage 3 Account Metrics Rebuild (Dev)

`daily_account_metrics` is the first persistent account-level aggregation layer for stage 3.

## What it rebuilds

For one `seller_account_id` and each `metric_date`:

- `revenue` from `sales.amount` grouped by `sale_date::date`
- `orders_count` from `orders` grouped by `created_at_source::date`
- `returns_count` from `orders.status = returned` (case-insensitive)
- `cancel_count` from `orders.status = cancelled` (case-insensitive)

If a status is absent or uses other values, it is not counted as return/cancel.

## Run rebuild

From repo root:

```bash
make dev-rebuild-account-metrics seller_account_id=1
```

Optional direct run with explicit date range:

```bash
cd backend
go run ./cmd/dev-rebuild-account-metrics --seller-account-id 1 --from 2026-02-01 --to 2026-03-31
```

Without `--from/--to`, service rebuilds across full available source date bounds for that seller account.

## Idempotency

Rebuild is idempotent for the target seller/date range:

- deletes existing `daily_account_metrics` rows in range for that seller;
- writes fresh aggregates with upsert;
- repeated runs do not create duplicates.
