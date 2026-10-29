# Stage 3: Dashboard Derived Metrics (MVP)

## Scope

- Added backend-only `DashboardService` that builds dashboard-ready DTO.
- Uses existing sources only:
  - `daily_account_metrics`
  - `daily_sku_metrics`
- No frontend/API handlers/ingestion changes.

## Account-level metrics

For `as_of_date` (default: current UTC date):

- `revenue_today`
- `revenue_yesterday`
- `revenue_last_7d` (sum from `as_of_date-6` to `as_of_date`)
- `revenue_day_to_day_delta.abs = revenue_today - revenue_yesterday`
- `revenue_day_to_day_delta.pct = (revenue_today - revenue_yesterday) / revenue_yesterday` (NULL when denominator is 0)
- `revenue_week_to_week_delta.abs = revenue_last_7d - previous_week_revenue`
- `revenue_week_to_week_delta.pct = (revenue_last_7d - previous_week_revenue) / previous_week_revenue` (NULL when denominator is 0)
- `orders_today`, `orders_yesterday`, `returns_today`, `cancels_today`

## SKU-level metrics

For each SKU at `as_of_date`:

- `revenue`
- `orders_count`
- `stock_available`
- `days_of_cover`
- `revenue_delta_day_to_day = revenue(today) - revenue(yesterday)`
- `orders_delta_day_to_day = orders(today) - orders(yesterday)`
- `share_of_revenue = sku_revenue / account_revenue_today` (NULL when `account_revenue_today = 0`)
- `contribution_to_revenue_change = revenue(today) - revenue(yesterday)`

## Dev check CLI

```bash
make dev-check-dashboard-metrics seller_account_id=1
```

Optional date:

```bash
cd backend
go run ./cmd/dev-check-dashboard-metrics --seller-account-id 1 --as-of-date 2026-04-20
```
