# Analytics metrics source map

Last audited: 2026-05-18 (control review: cancellation/units/stock aggregation/nullable).

This document maps each analytics metric to its data source, formula, null rules, and implementation status.
Metrics are exposed in API/UI/AI context only when `availability_status` is **current**.

## Status legend

| Status | Meaning |
|--------|---------|
| `current` | Source exists and metric is computed in analytics layer |
| `requires_quantity_storage` | `sales.quantity` exists but is not rolled into daily metrics yet (or column missing) |
| `requires_v1_analytics_stocks` | Needs `stock_analytics_snapshots` populated via `POST /v1/analytics/stocks` |
| `requires_v1_analytics_data` | Needs `POST /v1/analytics/data` ingestion (`product_analytics_daily`) |
| `requires_posting_status_storage` | Needs normalized posting fields (shipment deadline, delivery schema, etc.) |
| `requires_returns_ingestion` | Needs dedicated returns report/API, not order status proxy |
| `unavailable` | No source in DB or API |

## Null rules (global)

- All ratios: denominator `<= 0` → `null` (never `0`, `NaN`, or `Inf`).
- Missing source row → field omitted / `null`, not zero-filled.
- `orders_count` (posting-level) and `units_sold` (sale quantity) are distinct metrics.
- Nullable API fields use JSON `null` / omitted keys — not `0`, `NaN`, or `Infinity` (see **Nullable metrics** below).

---

## Nullable metrics (API / DTO)

These ratio fields are `*float64` / `omitempty` in Go and `null` in JSON when undefined:

`average_order_value`, `cancellation_rate`, `account_ad_spend_to_revenue`, `sku_orders_share`, `sku_revenue_share`, `sku_orders_delta_percent`, `sku_sales_velocity_units_7d`, `sku_sales_velocity_units_14d`, `discount_depth`, `excess_stock_share`, all `campaign_*` ratios (`campaign_drr`, `campaign_roas`, `campaign_cpo`, `campaign_ctr`, `campaign_cpc`, `campaign_click_to_order_rate`, `campaign_spend_share`).

`units_sold` / `units_sold_today` are omitted when the DB value is NULL (pre-rebuild history). A stored `0` after rebuild is a real zero, not “missing”.

---

## A. Account / SKU / ad / product pricing (current sources)

### 1. `average_order_value`

| Field | Value |
|-------|-------|
| **formula** | `revenue / orders_count` |
| **source_method** | `ListDailyAccountMetricsBySellerAndDateRange` |
| **source_table** | `daily_account_metrics` |
| **source_fields** | `revenue`, `orders_count` |
| **availability_status** | `current` |
| **null_rules** | `orders_count <= 0` → null |
| **caveats** | `orders_count` is Ozon posting count by `orders.created_at_source` date, not SKU sale-operation count. |

### 2. `sku_orders_share`

| Field | Value |
|-------|-------|
| **formula** | `sku_orders_count / account_orders_count` |
| **source_table** | `daily_sku_metrics.orders_count`, `daily_account_metrics.orders_count` |
| **availability_status** | `current` |
| **null_rules** | `account_orders_count <= 0` → null |
| **caveats** | SKU grain is sales-operation count; account grain is posting count — shares are indicative, not identical semantics. |

### 3. `sku_revenue_share`

| Field | Value |
|-------|-------|
| **formula** | `sku_revenue / account_revenue` |
| **source_table** | `daily_sku_metrics.revenue`, `daily_account_metrics.revenue` |
| **availability_status** | `current` |
| **null_rules** | `account_revenue <= 0` → null |

### 4. `sku_sales_velocity_orders_7d`

| Field | Value |
|-------|-------|
| **formula** | `SUM(daily_sku_metrics.orders_count, last 7 days incl. asOfDate) / 7` |
| **source_table** | `daily_sku_metrics` |
| **availability_status** | `current` |
| **null_rules** | Always computed as average over 7 calendar days (missing days = 0 in sum). |

### 5. `sku_sales_velocity_orders_14d`

| Field | Value |
|-------|-------|
| **formula** | `SUM(orders_count, 14d) / 14` |
| **source_table** | `daily_sku_metrics` |
| **availability_status** | `current` |

### 6. `sku_orders_delta_percent`

| Field | Value |
|-------|-------|
| **formula** | `(current_orders_count - previous_orders_count) / previous_orders_count * 100` |
| **source_table** | `daily_sku_metrics` |
| **availability_status** | `current` |
| **null_rules** | `previous_orders_count <= 0` → null |

### 7–13. Campaign ad metrics (`campaign_drr`, `campaign_roas`, `campaign_cpo`, `campaign_ctr`, `campaign_cpc`, `campaign_click_to_order_rate`, `campaign_spend_share`)

| Field | Value |
|-------|-------|
| **source_table** | `ad_metrics_daily` (aggregated per campaign in date range) |
| **source_fields** | `spend`, `revenue`, `orders_count`, `impressions`, `clicks` |
| **availability_status** | `current` |
| **null_rules** | See task spec per metric (e.g. `campaign_drr`: `revenue <= 0` → null; never force DRR=100% at zero revenue). |

### 14. `account_ad_spend_to_revenue`

| Field | Value |
|-------|-------|
| **formula** | `SUM(ad_metrics_daily.spend) / daily_account_metrics.revenue` |
| **availability_status** | `current` |
| **null_rules** | `account_revenue <= 0` → null |

### 15. `discount_depth`

| Field | Value |
|-------|-------|
| **formula** | `(old_price - current_price) / old_price` |
| **source_table** | `products` |
| **source_fields** | `old_price`, `reference_price` (current list price) |
| **availability_status** | `current` |
| **null_rules** | `old_price <= 0` or `reference_price` missing/`<= 0` → null |
| **caveats** | Analytics only. Price/economics **alerts remain disabled** even when this metric is available. |

---

## B. Quantity / units (`sales.quantity`)

### 16. `units_sold`

| Field | Value |
|-------|-------|
| **formula** | `SUM(sales.quantity)` with rollup filters (see below) per account day / SKU day |
| **source_table** | `sales` → `daily_sku_metrics.units_sold`, `daily_account_metrics.units_sold` |
| **source_fields** | `sales.quantity`, `sales.amount`, `sales.raw_attributes.operation_type` |
| **rollup_filters** | Include row only if `quantity > 0`, `amount > 0`, and `operation_type` does not contain `return` or `cancel` (case-insensitive). |
| **availability_status** | `current` with **partial** finance hygiene — not a certified Ozon “sold units” report. |
| **null_rules** | Not substituted with `orders_count`. Column is nullable; NULL until metrics rebuild (migration `000029`). |
| **caveats** | `ordersync` still ingests all finance operations into `sales`; rollup filters reduce noise but do not replace an allowlist by `operation_type`. One sale row with `quantity=3` yields `orders_count=1` (COUNT) and `units_sold=3` (SUM). |

**Rebuild after deploy:** run account + SKU daily metrics rebuild (`dev-rebuild-account-metrics`, `dev-rebuild-sku-metrics`, or worker post-sync) so historical `units_sold` is populated.

### 17–18. `sku_sales_velocity_units_7d` / `sku_sales_velocity_units_14d`

| Field | Value |
|-------|-------|
| **formula** | `SUM(units_sold, N days) / N` when every day in the window has a non-null `daily_sku_metrics.units_sold` for that SKU |
| **source_table** | `daily_sku_metrics.units_sold` |
| **availability_status** | `current` after rebuild; **null** if any day in the window has a SKU metric row with NULL `units_sold` (incomplete history). |
| **caveats** | **Must not** use `orders_count`. Missing calendar days without a SKU row count as zero units in the sum. |

---

## C. FBO stock analytics (`POST /v1/analytics/stocks`)

**Client:** `backend/internal/integrations/ozon/analytics_stocks_client.go`  
**Sync:** `stockanalyticssync` → `stock_analytics_snapshots` (migration `000028`)  
**SKU aggregation:** `aggregateStockAnalyticsBySKU` in `backend/internal/analytics/stock_analytics_aggregate.go` (warehouse rows → one row per SKU). Same rules as FBO alert aggregation.

| Field | Aggregation rule |
|-------|------------------|
| `available_stock_count`, `excess_stock_count` | **SUM** across warehouses |
| `idc` | **MAX** (worst / longest cover; never SUM) |
| `ads` | **AVG** of non-null warehouse `ads` (never SUM) |
| `days_without_sales` | **MAX** across warehouses |
| `turnover_grade` | **Worst grade** by priority: `SURPLUS` > `NO_SALES` > `DEFICIT` > `POPULAR` > `ACTUAL` > … |
| `excess_stock_share` | `SUM(excess) / SUM(available)`; null if `SUM(available) <= 0` |

### 19. `ozon_idc`

| **source** | MAX(`stock_analytics_snapshots.idc`) per SKU |
| **availability_status** | `current` when snapshot exists for as-of date |

### 20. `ozon_ads`

| **source** | AVG(`stock_analytics_snapshots.ads`) per SKU |
| **availability_status** | `current` when snapshot exists |

### 21. `stock_turnover_grade`

| **source** | Worst `turnover_grade` per SKU (see table above) |
| **availability_status** | `current` when snapshot exists |

### 22. `excess_stock_share`

| **formula** | `SUM(excess_stock_count) / SUM(available_stock_count)` per SKU |
| **null_rules** | `SUM(available_stock_count) <= 0` → null |

### 23. `days_without_sales`

| **source** | `stock_analytics_snapshots.days_without_sales` |
| **availability_status** | `current` when snapshot exists |

### 24. `stock_status_bucket`

| **formula** | Heuristic buckets from internal `days_of_cover` / `stock_available` and/or Ozon IDC/turnover/excess |
| **availability_status** | `current` (partial: `combined` when both internal + Ozon signals exist) |
| **buckets** | `out_of_stock`, `critical_shortage`, `shortage_7d`, `planning_14d`, `healthy`, `surplus_120d`, `no_sales` |
| **caveats** | `out_of_stock` is informational; stock alerts do not treat `current_stock <= 0` as a standalone trigger. Returns `stock_status_bucket_source`: `internal_days_of_cover`, `ozon_stock_analytics`, or `combined`. |

If `stock_analytics_snapshots` has no rows for the account/date → Ozon fields omitted (`requires_v1_analytics_stocks` for those fields only).

---

## D. Product funnel (`POST /v1/analytics/data`)

| Field | Value |
|-------|-------|
| **availability_status** | `requires_v1_analytics_data` |
| **caveats** | No client/ingestion in repo. Do not expose `GET /api/v1/analytics/product-funnel` until `product_analytics_daily` exists. |

Deferred metrics (25–32): `card_open_rate_proxy`, `pdp_to_cart_rate`, `search_to_cart_rate`, `total_to_cart_rate`, `cart_to_order_rate`, `view_to_order_rate`, `revenue_per_view`, `revenue_per_pdp_view`.

**Proposed module (not implemented):** migration `product_analytics_daily`, `productanalyticssync` service, Ozon `POST /v1/analytics/data` client with permission-denied / empty-response handling.

**Important:** `card_open_rate_proxy` is **not** exact CTR — do not describe as “клики” without a real clicks field.

---

## E. Cancellations / returns / fulfillment

### 33. `cancellation_rate`

| Field | Value |
|-------|-------|
| **formula** | `cancel_count / orders_count` |
| **source_table** | `daily_account_metrics` (from `orders` in rollup SQL) |
| **cancel_count** | `COUNT(*)` where `LOWER(orders.status) = 'cancelled'` |
| **orders_count** | `COUNT(*)` of **all** postings on `created_at_source` date (includes cancelled, returned, and active statuses) |
| **returns** | `status = 'returned'` → `returns_count` only; **not** included in `cancel_count` |
| **denominator** | Total postings (`orders_count`), **not** `orders_count - cancel_count` and **not** `orders_count + cancel_count` |
| **availability_status** | `current` |
| **null_rules** | `orders_count <= 0` → null |
| **caveats** | Interpretation: “share of postings that day which are cancelled”. Not net-of-cancel denominator. |

### 34. `return_rate`

| Field | Value |
|-------|-------|
| **availability_status** | `requires_returns_ingestion` |
| **caveats** | `daily_account_metrics.returns_count` uses `orders.status = 'returned'` — **not** exposed as `return_rate` (ambiguous vs real returns report). |

### 35. `orders_by_status`

| Field | Value |
|-------|-------|
| **source_table** | `orders.status` |
| **source_method** | `CountOrdersByStatusForDateRange` |
| **availability_status** | `current` |
| **caveats** | Status strings are Ozon-native; not normalized posting lifecycle. |

### 36. `shipment_delay_risk`

| Field | Value |
|-------|-------|
| **availability_status** | `requires_posting_status_storage` |
| **caveats** | `orders` lacks `shipment_date`, `delivery_schema` columns; FBS SLA not queryable reliably. |

---

## Explicit non-goals / warnings

1. **SKU-level advertising metrics are not supported.** No `sku_campaign_drr`, `sku_campaign_ctr`, `sku_campaign_spend`, `sku_campaign_roas` — only campaign-level `ad_metrics_daily` + SKU linkage via `ad_campaign_skus` for risk heuristics.

2. **`card_open_rate_proxy` is not exact CTR** (funnel block deferred).

3. **`return_rate` requires returns source** — not implemented from order status.

4. **Price/economics alerts remain disabled** even when `discount_depth` is available.

5. **Product funnel metrics** may be unavailable due to Ozon permissions / Premium analytics (not ingested).

6. **`units_sold` requires `sales.quantity`** — must not be substituted by `orders_count`; rollup uses partial finance filters; rebuild required for historical dates after migration `000029`.

7. **Frontend** does not yet bind new dashboard KPI fields (`average_order_value`, `cancellation_rate`, etc.); backend returns nullable JSON safely. Advertising UI already null-checks ROAS (`fmtRoas`).

---

## Source tables quick reference

| Table | Role |
|-------|------|
| `daily_account_metrics` | Account-day revenue, orders, returns, cancels, units_sold |
| `daily_sku_metrics` | SKU-day revenue, orders, stock, days_of_cover, units_sold |
| `sales` | Raw sales + `quantity` |
| `orders` | Posting status counts |
| `products` | `old_price`, `reference_price` |
| `ad_metrics_daily` | Campaign-day ad stats |
| `stock_analytics_snapshots` | Ozon FBO analytics stocks API |
| `product_analytics_daily` | **Not created** |
