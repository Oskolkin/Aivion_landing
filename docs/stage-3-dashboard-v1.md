# Stage 3: Dashboard v1 API + Frontend

## Backend routes

- `GET /api/v1/analytics/dashboard`
- `GET /api/v1/analytics/sku-table`
- `GET /api/v1/analytics/stocks`

## Query params

- `dashboard`: `as_of_date=YYYY-MM-DD` (optional)
- `sku-table`:
  - `as_of_date=YYYY-MM-DD` (optional)
  - `limit` (optional, default 20, max 100)
  - `offset` (optional, default 0)
  - `sort_by` (optional: `revenue`, `orders_count`, `share_of_revenue`, `contribution_to_revenue_change`, `stock_available`, `days_of_cover`, `product_name`)
  - `sort_order` (optional: `asc`/`desc`, default `desc`)
- `stocks`: no query params in MVP

## Stocks response format

- Flat warehouse rows for table usage:
  - `ozon_product_id`, `offer_id`, `sku`, `product_name`
  - `warehouse`
  - `quantity_total`, `quantity_reserved`, `quantity_available`
  - `snapshot_at`

## Manual check

1. Start backend API and frontend.
2. Log in.
3. Open `/app/dashboard`.
4. Verify data via API:
   - `/api/v1/analytics/dashboard`
   - `/api/v1/analytics/sku-table?limit=20&offset=0&sort_by=revenue&sort_order=desc`
   - `/api/v1/analytics/stocks`
