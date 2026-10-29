# Stage 7 Alert Engine Service

`RunForAccount` in `backend/internal/alerts/service.go` is the orchestration entrypoint for Alert Engine execution on one seller account.

## What it runs

Execution order:

1. sales alerts (`RunSalesAlerts`)
2. stock alerts (`RunStockAlerts`)
3. advertising alerts (`RunAdvertisingAlerts`)
4. price/economics alerts (`RunPriceEconomicsAlerts`)

Each group upserts alerts into `alerts` using fingerprint-based idempotency.

## How `alert_runs` is used

`RunForAccount` creates and updates run journal records in `alert_runs`:

1. `CreateRun` with `status=running` and configured `run_type`
2. execute all groups
3. if success: `CompleteRun` with per-group counts and total count
4. if any group fails: `FailRun` with error message

Partial upserts done before an error are kept (no manual rollback in MVP).

## Deferred behavior

Automatic resolving of stale/missing alerts is intentionally deferred in MVP.
Dismissed alert behavior remains unchanged by orchestration.
