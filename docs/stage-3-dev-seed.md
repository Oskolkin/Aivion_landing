# Stage 3 Dev Synthetic Seed

Developer-only synthetic seed for local validation of dashboard and future aggregation layer.

## Purpose

- Populate realistic synthetic source data for one selected `seller_account_id`.
- Keep production ingestion behavior unchanged.
- Allow reproducible local checks when real Ozon account data is empty.

## Command

From repository root:

```bash
make dev-seed-stage3 seller_account_id=1
```

Equivalent direct run:

```bash
cd backend
go run ./cmd/dev-seed-stage3 --seller-account-id 1
```

`seller_account_id` is required and must already exist in `seller_accounts`.

## What seed does

For the selected seller account only:

1. Cleans previous synthetic/operational rows in:
   - `products`
   - `orders`
   - `sales`
   - `stocks`
2. Generates deterministic synthetic dataset:
   - 50 products
   - 60-day orders history
   - linked sales history with leaders and long tail
   - current stock snapshot with out-of-stock / low-stock / high-stock cases

Seed does not write to:

- `raw_payloads`
- `sync_jobs`
- `import_jobs`
- `sync_cursors`
- analytical tables (`daily_account_metrics`, `daily_sku_metrics`)

## Deterministic behavior

- Fixed anchor date and fixed random seed are used.
- Same `seller_account_id` produces the same data structure on repeated runs.
- Re-run is safe for dev flow because data is re-created for that seller account in one transaction.

## Safety notes

- The tool is not called by backend startup and never runs automatically.
- It modifies data only for explicitly provided `seller_account_id`.
- It is intended for local/dev use only.
