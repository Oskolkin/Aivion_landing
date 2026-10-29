# AI Daily Summaries

## Purpose

AI Daily Summaries are **management-style daily analytical briefings** for an Ozon seller cabinet. They complement (and do not replace) alert-driven SKU **recommendations**.

- **Account summary** (`account_daily_summary`): one briefing for the latest available `metric_date`.
- **Category summaries** (`category_daily_summary`): one briefing per included merchandise category (Ozon `description_category_id`).

Summaries are **not** alerts and **not** persisted as `recommendations` rows. Action items live inside `summary_json`.

## Generation frequency

Summaries are generated **asynchronously** after ingestion post-sync recalculation completes (bounded metrics rebuild + alerts), via an Asynq task (`ai.generate_daily_summaries`).

There is **no** user-facing “generate now” button and **no** synchronous generation on dashboard load.

### Local / dev manual generation

For demo and local smoke tests only:

```bash
cd backend
go run ./cmd/dev-generate-ai-summaries --seller-account-id=1
```

Requires `OPENAI_API_KEY` in `backend/.env.local`, seeded metrics (`dev-seed-mvp`), and the same Postgres as the API. The CLI calls `aisummaries.Service.RunForSellerAfterSync` directly (no HTTP endpoint, no worker bypass). See `docs/dev-seed-mvp.md` §2d.

## Latest `metric_date` logic

Generation uses `GetLatestAvailableDashboardMetricDateBySellerAccountID` (same semantics as the dashboard): `MAX(daily_account_metrics.metric_date)` for the seller, falling back to `MAX(daily_sku_metrics.metric_date)` only when account metrics are absent (aligned with MVP seed validation anchor).

If that date is missing / sentinel-empty (no real metrics yet), generation is skipped and the API returns `not_enough_data`.

## Previous summary context

For each new summary, the service loads the latest **completed** prior summary of the same scope:

- Account: previous `account_daily_summary` for the same seller with `metric_date < current`.
- Category: previous `category_daily_summary` for the same `(seller, category_id)` with `metric_date < current`.

A **shortened** JSON snapshot is passed as `previous_summary_context`. The model is instructed that it is **context, not truth** and must be verified against current metrics.

## Category inclusion rule

Categories are included when the rollup query finds at least one **non-archived** product with `description_category_id` set and **positive** `stock_available` on the target `metric_date` (from `daily_sku_metrics`).

## Depth / `data_richness`

The context builder adds `inferred_data_richness_hint` (`low|medium|high`). The model must align `data_richness` and `summary_depth` (`short|normal|deep`) with available evidence.

## Category names

Category names come from the global `description_categories` dictionary populated from Ozon Seller API:

- `POST /v1/description-category/tree`

The tree is synced on a **throttled** schedule (via `internal_meta`) during product import (best-effort background) and again before summary generation.

If a name is missing, the UI/API uses **“Категория без названия”** and adds a `data_quality_note`.

## Category ↔ ads linkage

Category-level ad metrics are **not** aggregated in MVP (SKU↔campaign linkage is not treated as reliable for category DRR/ROAS). The category context includes an explicit note telling the model not to invent category ad efficiency.

## Structured JSON schema

The model returns a single JSON object; see `backend/internal/aisummaries/schema.go` and prompt text in `prompt.go`.

## Dashboard usage

Frontend can call:

- `GET /api/v1/ai-summaries/latest` (requires `connection_ready`)

Response fields include `status` (`completed|partial|pending|failed|not_enough_data`), optional `account_summary`, and `category_summaries[]`.

## Run observability

`ai_daily_summary_runs` tracks batch status:

- `pending | running | completed | partial | failed`
- per-run token/cost estimates (sum of child calls in MVP)
- `category_total_count`, `category_completed_count`, `category_failed_count`

## Limitations (MVP)

- No exports / PDF.
- No automatic creation of `recommendations` rows from summary actions.
- No price-management actions (prompt guardrails + validator warnings).
- `input_context_json` is stored **truncated** when large.
- If OpenAI is unavailable, summaries may fail; ingestion still completes.

## Token / context budget

Summaries reuse the same OpenAI Responses transport as recommendations (`GenerateStructuredJSON`) and honor `AI_MAX_INPUT_TOKENS_APPROX` / `AI_MAX_OUTPUT_TOKENS` from config.

Account/category context packages are additionally clamped in Go before the model call.

## Partial generation

If the account summary completes but some category summaries fail validation or OpenAI, the run is marked **`partial`** and the latest API returns whatever completed rows exist.

## Idempotency

A **completed** run for `(seller_account_id, metric_date)` prevents re-generation. Failed/partial/in-progress runs are cleaned up and regenerated on the next enqueue for the same metric date.
