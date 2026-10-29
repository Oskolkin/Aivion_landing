# Dashboard Home Redesign

## Purpose

The seller app main page (`/app/dashboard`) is a **daily sales control center**, not a table-heavy BI report. It answers in ~30 seconds:

1. What is the account state?
2. Where are sales bottlenecks?
3. Which categories and SKUs need attention?
4. What to do today?

## Four sections

| Section | Backend block | Primary sources |
|---------|---------------|-----------------|
| AI-диагностика продаж | `ai_diagnostics` | AI daily summaries, product funnel, SKU stock buckets, ad campaigns, alerts |
| Пульс селлера | `seller_pulse` | `daily_account_metrics`, alerts severity counts, ad DRR |
| SKU-фокус | `sku_focus` | `daily_sku_metrics`, replenishment, critical SKUs, products images |
| Рекомендуемые действия | `recommended_actions` | AI summary actions, open recommendations, high/critical alerts |

## API contract

`GET /api/v1/dashboard/home` (connection_ready gated)

- Global `status`: `ready` | `partial` | `not_enough_data`
- Per-widget `status`: `available`, `not_enough_data`, `source_not_available`, `source_access_denied`, `premium_required`, `source_pending`, `error`
- Never returns 500 for missing optional data; uses `partial` + `data_quality_notes`

## Visualization principles

- No large tables on the main page
- No chart library dependency — lightweight SVG/HTML widgets
- Each widget maps to a management question
- Informative empty states instead of blank charts
- No technical sync/job status on the dashboard
- No price management, manual AI generation, or export on this page

### AI summary pending on local demo

If `ai_diagnostics.account_summary` shows **source_pending** / «AI-сводка появится после обработки последних метрик», metrics may exist but **AI daily summaries** were never generated. In production they run via worker after post-sync recalc; locally use:

```bash
cd backend
go run ./cmd/dev-generate-ai-summaries --seller-account-id=1
```

(`OPENAI_API_KEY` required — see `docs/dev-seed-mvp.md`.)

## Sales funnel — Ozon `POST /v1/analytics/data`

**Confirmed metrics** (Ozon Seller API `analyticsMetric` enum):

| Funnel stage | Ozon metric |
|--------------|-------------|
| Показы | `hits_view` |
| Просмотры карточки | `hits_view_pdp` |
| Добавления в корзину | `hits_tocart` |
| Заказанные единицы | `ordered_units` |
| Доставленные единицы | `delivered_units` (Ozon analyticsMetric, Premium) |
| Выручка (aux) | `revenue` |

**Implementation status:**

- Ozon client: `backend/internal/integrations/ozon/analytics_data_client.go`
- Storage: `product_analytics_daily` (migration `000034`)
- Ingestion: `backend/internal/productanalyticssync` (account-level, dimension `day`)
- Dashboard widget reads stored rows; if none → `sales_funnel.status = source_not_available`

Funnel is **real-data-backed when ingestion has run**; otherwise **status-backed** with user-facing message.

Access denied / premium responses from Ozon are persisted in `product_analytics_daily.source_status` during post-sync funnel sync (non-blocking). Dashboard maps them to `source_access_denied` / `premium_required`.

## Frontend

- Single API: `frontend/lib/dashboard-home-api.ts`
- Types: `frontend/lib/dashboard-home-types.ts`
- Screen: `frontend/components/dashboard-screen.tsx`
- Section components under `frontend/components/dashboard/`

Legacy analytics endpoints (`/api/v1/analytics/*`) remain for Products, Advertising, Alerts, Recommendations screens.

## Pre-E2E hardening (MVP)

- **Price/economics alerts**: generation disabled (`PriceEconomicsGenerationDisabled`); legacy rows may exist in DB but are not promoted in AI summary context or seller filters.
- **SKU sales drop**: active type `sku_orders_drop` (orders day-over-day); `sku_revenue_drop` is legacy-only.
- **Risk distribution**: each open alert maps to exactly one bucket via `alerts.AlertRiskBucket` (type-first, then group). No double-count across `stock` + `surplus` / `critical_sku`.
- **Category cards**: missing names use **«Категория без названия»**.
- **Product funnel**: post-sync syncs `yesterday + today`; `delivered_units` label **«Доставленные единицы»**; funnel ends at **«Заказанные единицы»** when delivered is absent.
