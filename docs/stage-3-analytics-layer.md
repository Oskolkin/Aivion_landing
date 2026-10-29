# Stage 3: Backend Analytics Layer

## Services in `backend/internal/analytics`

- `account_metrics_service.go`
  - rebuilds `daily_account_metrics` from source tables.
- `sku_metrics_service.go`
  - rebuilds `daily_sku_metrics` and calculates stored SKU metric fields.
- `dashboard_service.go`
  - reads daily metrics and produces derived dashboard DTO (`dashboard v1` ready).
- `stocks_view_service.go`
  - builds current stocks table view for seller account.

## Separation from ingestion

- Analytics layer is read/rebuild oriented.
- No ingestion runtime changes.
- No background orchestration in MVP.
- Rebuild/check flow is manual via existing dev commands.

## Manual dev checks

```bash
make dev-rebuild-account-metrics seller_account_id=1
make dev-rebuild-sku-metrics seller_account_id=1
make dev-check-stock-metrics seller_account_id=1
make dev-check-dashboard-metrics seller_account_id=1
```
