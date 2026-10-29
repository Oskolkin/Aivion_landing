# Stage 3 Stock Metrics Layer (Current-State View)

This step implements stock metrics as a service/query layer over existing `stocks`, without creating a new persistent `stock_metrics` table.

## Scope

- Current-state stock view only.
- No new migrations.
- No API handlers.
- No frontend changes.

## Data semantics

- `stocks` is treated as snapshot source.
- For each `(product_external_id, warehouse_external_id)`, latest snapshot is selected deterministically:
  - `ORDER BY snapshot_at DESC NULLS LAST, id DESC`.
- Product identity is aligned with SKU analytics:
  - `seller_account_id`
  - `ozon_product_id` (derived from `product_external_id`)
  - optional metadata from `products`: `offer_id`, `sku`, `product_name`.

## Service output format

The service returns product-level summary rows with nested warehouse breakdown:

- product summary:
  - `ozon_product_id`
  - `offer_id`
  - `sku`
  - `product_name`
  - `warehouse_count`
  - `total_stock`
  - `reserved_stock`
  - `available_stock`
  - `snapshot_at` (max snapshot among current warehouse rows)
- nested `warehouses[]`:
  - `warehouse_external_id`
  - `total_stock`
  - `reserved_stock`
  - `available_stock`
  - `snapshot_at`

This format is intentionally chosen as minimal MVP that is directly usable by a future frontend stock table.

## Dev check

From repo root:

```bash
make dev-check-stock-metrics seller_account_id=1
```

Direct run:

```bash
cd backend
go run ./cmd/dev-check-stock-metrics --seller-account-id 1
```
