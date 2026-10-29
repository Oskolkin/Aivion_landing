# Stage 3 SKU Metrics Rebuild (Dev)

`daily_sku_metrics` is the SKU-level persistent aggregation layer for stage 3.

## Source model and MVP rule

- SKU-day source uses `sales` as primary grain.
- SKU identity is extracted from `sales.raw_attributes.product_id` (fallback `ozon_product_id` in raw attributes), then joined with `products` for `offer_id`, `sku`, `product_name`.
- This is an explicit MVP tradeoff: current normalized source tables do not provide a strict direct `orders -> product` key for robust SKU-day counting.
- `orders_count` is currently counted as number of sales operations per SKU/day.

## Stored fields in `daily_sku_metrics`

- `revenue`
- `orders_count`
- `stock_available`
- `days_of_cover`
- identity/display fields: `ozon_product_id`, `offer_id`, `sku`, `product_name`

Derived fields for dashboard logic are not persisted at this step:

- `share_of_revenue` = `sku_revenue / account_revenue` for same day
- `contribution_to_result_change` = `sku_revenue(day) - sku_revenue(day-1)`

Both are deterministic and can be recomputed during rebuild/dashboard layer later.

## Days of cover rule

For each SKU-day row:

- `days_of_cover = stock_available / avg_daily_orders_recent_7_days`
- `avg_daily_orders_recent_7_days` is average of SKU `orders_count` over last 7 days including current day
- if average demand is `0`, `days_of_cover` is `NULL`

`stock_available` comes from current stock snapshot (latest per product+warehouse, then summed across warehouses).

## Run rebuild

From repo root:

```bash
make dev-rebuild-sku-metrics seller_account_id=1
```

Direct run with optional date range:

```bash
cd backend
go run ./cmd/dev-rebuild-sku-metrics --seller-account-id 1 --from 2026-02-01 --to 2026-03-31
```

Without range flags, rebuild uses full available SKU source date bounds.

## Idempotency

Rebuild is idempotent for seller/date range:

- delete existing `daily_sku_metrics` rows for seller in range;
- upsert recalculated rows;
- repeated runs do not create duplicates.
