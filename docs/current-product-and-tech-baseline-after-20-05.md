# Current product & tech baseline (post 20.05 hardening)

**Source of truth** for Cursor and developers. Do not re-enable removed seller-facing modules or legacy alert generation without an explicit product decision.

Related: [README](../README.md), [validation checklists](./validation/).

## Active MVP modules (seller-facing)

| Module | Notes |
|--------|--------|
| Ozon connection | `GET/PUT /api/v1/integrations/ozon/connection`, `POST .../disconnect` |
| Dashboard home | `GET /api/v1/dashboard/home` — partial statuses, deterministic KPIs, action dedup |
| Products | Catalog + sparklines (missing `daily_sku_metrics` days ≠ zero) |
| Alerts / Signals | Active groups: sales, stock, advertising |
| AI daily summaries | Interpretive layer; category cap per run |
| Recommendations | Validated action layer; daily run limits |
| AI Chat | Controlled pipeline; precheck; daily question limits |
| Admin / support | Technical diagnostics only (`ADMIN_EMAILS`) |

## Removed / deprecated (user-facing)

- **Pricing constraints screen** — removed; `/app/pricing*` redirects to Products.
- **Technical sync/status** — not seller-facing (`/app/sync-status` etc.).
- **Price/economics alert generation** — disabled (`PriceEconomicsGenerationDisabled`).

## Backend future / deprecated

- `backend/internal/pricingconstraints` — API may remain; not primary seller workflow.
- Legacy alert **types** remain in DB for history/admin reads.

## Active alert types (generated)

From `backend/internal/alerts/generation.go` → `activeAlertTypes`:

- `sales_revenue_drop`, `sales_orders_drop`
- `sku_orders_drop`, `sku_negative_contribution`
- `stock_low_coverage`, `stock_oos_risk`, `stock_critical_sku_low_stock`
- `fbo_surplus_storage_risk`
- `ad_weak_campaign_efficiency` (**campaign-level only**)

## Legacy alert types (not generated)

- Sales: `sku_revenue_drop`
- Advertising (SKU-level): `ad_spend_without_result`, `ad_budget_on_weak_sku`, `ad_budget_on_low_stock_sku`, `stock_advertised_sku_low_stock`
- Price/economics: `price_below_min_constraint`, `price_above_max_constraint`, `margin_risk_at_current_price`, `margin_risk_at_min_price`, `missing_pricing_constraints_for_key_sku`

Frontend: `price_economics` group is **legacy-only** (`frontend/lib/alert-labels.ts`).

## Advertising rule

Active MVP advertising alerts are **campaign-level only**. SKU-level ad spend is not attributed reliably in current sources — do not activate SKU-level ad alert generation or imply SKU-level ad spend in AI context without a data source.

## AI boundaries

| Layer | Role |
|-------|------|
| Deterministic metrics | Source of truth: KPI, stock, ads aggregates, funnel stages (when available), risk counts |
| AI daily summary | Interpretation: headline, health_score (diagnostic), text, suggested actions |
| Recommendations | Validated actions linked to alerts |
| Chat | Questions over prepared context; precheck; no OpenAI on reject/limit |

Dashboard action feed: dedup priority `recommendation > ai_summary > alert`.

## Ozon APIs used (high level)

- Seller API — products, orders, stocks, connection
- Performance API — campaigns, ad metrics
- Analytics — product/funnel (`product_analytics_daily`), stocks where integrated
- Category tree — description categories for AI summaries (when synced)

## Dashboard home

- Backend aggregator `dashboardhome.Service`
- Widget statuses: `available`, `not_enough_data`, `source_not_available`, `source_access_denied`, `premium_required`, `source_pending`, `error`
- Global `partial` when some blocks fail
- No heavy full-table scans in hot path
- Funnel: graceful states from `product_analytics_daily.source_status` — no fake zeros

## Billing (MVP)

- **Not a full payment/subscription system.** `seller_billing_state` is admin-managed placeholder (plan, token counters, notes).
- No external payment provider in repo.
- SKU/AI limits are **technical guards** (`AI_*_MAX_*` env), not commercial enforcement.

## Operational notes

| Topic | Config / behavior |
|-------|-------------------|
| Worker scheduler | `WORKER_SCHEDULER_ENABLED` — one replica should run periodic sync ticker |
| AI category cap | `AI_SUMMARY_MAX_CATEGORIES_PER_RUN` (default 10) |
| AI daily limits | `AI_CHAT_MAX_QUESTIONS_PER_DAY`, `AI_RECOMMENDATION_MAX_RUNS_PER_DAY`, `AI_SUMMARY_MAX_RUNS_PER_DAY` |
| Token caps | `AI_MAX_INPUT_TOKENS_PER_REQUEST`, `AI_MAX_OUTPUT_TOKENS_PER_REQUEST` |
| Raw AI / debug | Admin-only + audit (`view_raw_ai_payload`); seller DTOs omit `raw_ai_response` |
| Disconnect retention | `DISCONNECTED_ACCOUNT_RETENTION_DAYS` (default 90); cleanup optional |
| Sparklines | Missing metric day → null/missing point; `orders_count=0` with row → real zero |

## Do not revert without review

- Price/economics alerts or pricing UI
- SKU-level advertising alerts as active
- AI summary driving dashboard KPI numbers
- Seller access to raw OpenAI payloads
