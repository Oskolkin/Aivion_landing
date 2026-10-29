# dev-seed-mvp

CLI and **Make targets** for loading **deterministic, seller-scoped source data** into Postgres for a **full functional MVP test before Billing**. Data simulates Ozon Seller API + Performance-style payloads after ingestion/mapping (no live Ozon calls).

### Anchor date and history window

- **`--anchor-date today`** (default): the dataset ends on the **UTC calendar day when you run the seed** (not a fixed calendar constant).
- **`--anchor-date YYYY-MM-DD`**: the dataset ends on that date (useful for reproducible tests, e.g. `2026-05-21`).
- **`--days N`**: inclusive range `anchor - (N-1)` … `anchor` — exactly **N** distinct metric days; seed does **not** create orders, sales, or rebuilt metrics **after** the anchor.
- After seed, the CLI prints `anchor_date`, `generated_date_start`, `generated_date_end`, `expected_days`, and `future_business_rows_found` (should be `0`).

## Prerequisites

1. **Infrastructure** (from repo root):

   ```bash
   docker compose up -d postgres redis minio
   ```

   Optional: add other compose services if your local stack needs them.

2. **Migrations applied** (Postgres reachable at `DATABASE_URL` from backend config):

   ```bash
   make migrate-up
   ```

3. **Backend environment** (`backend/.env` or env vars loaded by `internal/config`):

   - `DATABASE_URL` — same Postgres as migrations.
   - **`ADMIN_EMAILS=admin@example.com`** — required for admin login (`admin@example.com` has no `seller_account`; access is allowlist-based).
   - **`ENCRYPTION_KEY`** — exactly **32 bytes**; must match the API/worker so `dev-seed-mvp` can encrypt Ozon credentials the same way the app decrypts them.
   - S3/MinIO variables if you exercise raw payload flows (seed does not require raw payloads).

4. **OpenAI (optional but required for AI steps in the manual flow)**  
   Add **`OPENAI_API_KEY`** (and related model settings per your `config`) to `.env` so **Recommendations** and **Chat** work when you run those screens.

## Quick start (Make)

Variables (all optional; defaults shown):

| Variable | Default |
|----------|---------|
| `SEED_EMAIL` | `demo@example.com` |
| `SEED_PASSWORD` | `password123` |
| `SEED_ADMIN_EMAIL` | `admin@example.com` |
| `SEED_SELLER_NAME` | `Demo Ozon Seller` |
| `SEED_ANCHOR_DATE` | `today` |
| `SEED_PRODUCTS` | `80` |
| `SEED_DAYS` | `90` |
| `SEED_BASE` | `20260514` |
| `SEED_RESET_PASSWORD` | `true` |
| `VALIDATE_SELLER_ACCOUNT_ID` | `1` (for `seed-mvp-validate` only) |

Targets:

| Target | Purpose |
|--------|---------|
| `make seed-mvp` | Full seed for a **new** demo seller (`--reset=true`). |
| `make seed-mvp-reset` | Same as `seed-mvp` (explicit name for scripts/docs that stress a clean re-seed). |
| `make seed-mvp-validate` | Seed-level DB checks for `VALIDATE_SELLER_ACCOUNT_ID` (no writes). |
| `make seed-mvp-validate-alerts` | Runs **production** Alerts Engine on `VALIDATE_SELLER_ACCOUNT_ID`; asserts alerts from seeded data (no seed writes). |
| `make seed-mvp-validate-alerts-reset` | Same as above with `--reset-derived=true` (clears `alerts` + `alert_runs` for that seller first). |
| `make seed-mvp-validate-recommendations` | Runs **production** recommendation generator (OpenAI) on `VALIDATE_SELLER_ACCOUNT_ID`; requires **open alerts** and **`OPENAI_API_KEY`**. |
| `make seed-mvp-validate-derived` | Read-only: checks that **manual** Alerts / Recommendations / Chat / admin flows left rows in DB for `VALIDATE_SELLER_ACCOUNT_ID`. |
| `make seed-mvp-existing SELLER_ACCOUNT_ID=1` | Re-seed an **existing** seller account (requires `SELLER_ACCOUNT_ID`; `--reset=true`). |

All targets run:

`cd backend && go run ./cmd/dev-seed-mvp ...`

By default **`--reset-password=true`**: if `demo@example.com` / `admin@example.com` (or the seller owner when using `--seller-account-id`) already exist, their `password_hash` is overwritten so login matches `--password` (e.g. `password123`). Set `SEED_RESET_PASSWORD=false` in Make (or pass `--reset-password=false`) to keep existing hashes.

### 1. Seed

```bash
make seed-mvp
```

Note the printed **`seller_account_id`** if it is not `1`, then set `VALIDATE_SELLER_ACCOUNT_ID` for validate, or use `seed-mvp-existing` with that id.

### 2. Validate

```bash
make seed-mvp-validate
# or, if your seller id is not 1:
make seed-mvp-validate VALIDATE_SELLER_ACCOUNT_ID=42
```

See [Validation](#validation-details) for exit codes and the report format.

### 2a. Validate alert generation (production Alerts Engine)

After seed + analytics rebuild, this mode runs **`alerts.Service.RunForAccountWithType`** with **`run_type=manual`**, the same entry point as **`POST /api/v1/alerts/run`** (no HTTP; no OpenAI). It asserts that **seeded source data** produces non-empty alert groups (sales, stock, advertising, price/economics) including at least one **high** or **critical** open alert.

```bash
cd backend
go run ./cmd/dev-seed-mvp --seller-account-id 1 --validate-alert-generation
# optional: wipe this seller's alerts + alert_runs first, then run the engine
go run ./cmd/dev-seed-mvp --seller-account-id 1 --validate-alert-generation --reset-derived=true
```

Or with Make (same `VALIDATE_SELLER_ACCOUNT_ID` as seed validate):

```bash
make seed-mvp-validate-alerts
make seed-mvp-validate-alerts-reset   # adds --reset-derived=true
```

Details: [Validate alert generation](#validate-alert-generation-details).

### 2b. Validate recommendation generation (production AI engine)

Runs **`recommendations.Service.GenerateForAccount`** (same stack as **`POST /api/v1/recommendations/generate`**: context builder → OpenAI → validator → DB). **Requires**:

- at least one **open** alert for the seller (run **`--validate-alert-generation`** or **Run alerts** in the UI first);
- **`OPENAI_API_KEY`** (and model settings) in the same env as the API server.

```bash
cd backend
go run ./cmd/dev-seed-mvp --seller-account-id 1 --validate-recommendation-generation
```

Or: `make seed-mvp-validate-recommendations` (uses `VALIDATE_SELLER_ACCOUNT_ID`).

Details: [Validate recommendation generation](#validate-recommendation-generation-details).

### 2d. Generate AI daily summaries (dev-only CLI)

For local smoke/demo when the dashboard shows **«AI-сводка появится после обработки последних метрик»**, you can run the **same** `aisummaries.Service.RunForSellerAfterSync` used by the worker task `ai.generate_daily_summaries` — without a full Ozon sync or Redis worker.

**Not for production:** production generation is enqueued after post-sync recalculation in the worker.

**Prerequisites:**

1. `make seed-mvp` (or `seed-mvp-existing`) and metrics rebuild (`dev-rebuild-account-metrics` / `dev-rebuild-sku-metrics` if needed).
2. **`OPENAI_API_KEY`** in `backend/.env.local` (command exits with `OPENAI_API_KEY is required to generate AI summaries` if missing).
3. **`DATABASE_URL`** and other backend config loaded like `cmd/api` / `cmd/worker`.

**Run:**

```bash
cd backend
go run ./cmd/dev-generate-ai-summaries --seller-account-id=1
```

Optional:

```bash
go run ./cmd/dev-generate-ai-summaries --seller-account-id=1 --dry-run
```

`--dry-run` prints seller, `metric_date`, OpenAI model, and estimated category count — no OpenAI calls, no DB writes.

**Verify:**

- `GET /api/v1/ai-summaries/latest` (seller session, `connection_ready`)
- http://localhost:3000/app/dashboard — AI diagnostics block should show account summary and «Что изменилось» when the run is `completed` or `partial`.

Idempotency matches the service: a **completed** or **running** run for the same `(seller_account_id, metric_date)` is skipped with a clear message; **failed** / **partial** runs are retried on the next invocation.

`dev-seed-mvp` also upserts **`description_categories`** names for MVP commerce taxonomy (so AI category summaries and dashboard do not show only «Категория без названия»). Re-run seed on an existing seller if category names were missing.

`--force` is not implemented (would require explicit confirmation before deleting completed runs).

### 2c. Validate derived (post–manual test, read-only)

After you have exercised **Alerts**, **Recommendations**, **Chat**, and **Admin** in the UI (or equivalent), this mode **only reads Postgres** and checks that expected tables have rows for the seller. It does **not** call OpenAI, does **not** run engines, and does **not** insert data.

```bash
cd backend
go run ./cmd/dev-seed-mvp --seller-account-id 1 --validate-derived
```

Or: `make seed-mvp-validate-derived`.

Details: [Validate derived](#validate-derived-details).

### 3. Login (default seed users)

| Role | Email | Password | After login |
|------|-------|----------|-------------|
| Seller | `demo@example.com` | `password123` | `/app` (seller UI) |
| Admin | `admin@example.com` | `password123` | `/app/admin` (admin/support UI; no `seller_account`) |

Set **`ADMIN_EMAILS=admin@example.com`** in the backend API environment so admin login is recognized.

(Override via `SEED_*` variables when running Make. Use `SEED_RESET_PASSWORD=false` if you must preserve passwords for existing accounts.)

After a successful seed, the CLI prints **`Password updated: demo=… admin=…`**: `true` means this run wrote a new bcrypt hash for that account (including newly created users).

### 4. Manual test flow (pre-billing MVP)

Walk through in order (or as needed):

| Step | Route | What to verify |
|------|-------|----------------|
| A | `/app` | MVP home / navigation. |
| B | `/app/integrations/ozon` | Connection + Performance status after seed. |
| C | `/app/sync-status` | Sync jobs, import jobs, cursors. |
| D | `/app/dashboard` | Metrics and widgets fed by `daily_*` rebuild. |
| E | `/app/critical-skus` | Critical SKU logic on seeded catalog. |
| F | `/app/stocks-replenishment` | Replenishment views on seeded stocks. |
| G | `/app/advertising` | Campaigns / metrics from seeded ads. |
| H | `/app/pricing-constraints` | Rules + effective constraints from seed. |
| I | `/app/alerts` | Click **Run alerts** — creates alerts from seeded economics/ads/pricing. |
| J | `/app/recommendations` | **Generate recommendations** (after alerts if your flow expects it). |
| K | `/app/chat` | Ask real questions (requires OpenAI key). |
| L | `/app/admin` | Client detail, sync/import, billing stub, action logs / diagnostics as applicable. |

### 5. Suggested chat questions (Russian)

Use in `/app/chat` after OpenAI is configured:

- Что мне сделать сегодня в первую очередь?
- Почему упали продажи?
- Какие товары заканчиваются?
- Где реклама работает хуже всего?
- Какие SKU опасно рекламировать?
- Почему система советует проверить цену?
- Какие товары требуют пополнения?
- Где я теряю больше всего денег?

### 6. Important: what the seed does **not** create

**`dev-seed-mvp` does not insert** `alerts`, `alert_runs`, `recommendations`, `recommendation_runs`, `chat_sessions`, `chat_messages`, `chat_traces`, or related feedback/history during **seed**. Those are **created by the real MVP features** when you run Alerts, Generate recommendations, and Chat during testing — except **`--validate-alert-generation`** (writes `alerts` / `alert_runs` via the alerts engine), **`--validate-recommendation-generation`** (writes recommendation rows and runs via the real OpenAI-backed generator), and **`--validate-derived`** which **never writes** (it only checks that you already created those artifacts).

With `--reset=true`, old rows for those tables **for the same seller** may be deleted so the next run stays clean; the seed still does not fabricate AI or alert rows.

---

## Run (without Make)

From the repository root (with `DATABASE_URL` / config used by `internal/config`):

```bash
cd backend
go run ./cmd/dev-seed-mvp
```

Common flags:

```bash
go run ./cmd/dev-seed-mvp --email demo@example.com --password password123 --reset=true
go run ./cmd/dev-seed-mvp --seller-account-id 42 --reset=true
go run ./cmd/dev-seed-mvp --seller-account-id 42 --reset=true --reset-password=false
go run ./cmd/dev-seed-mvp --seller-account-id 1 --validate-only
go run ./cmd/dev-seed-mvp --seller-account-id 1 --validate-alert-generation
go run ./cmd/dev-seed-mvp --seller-account-id 1 --validate-alert-generation --reset-derived=true
go run ./cmd/dev-seed-mvp --seller-account-id 1 --validate-recommendation-generation
go run ./cmd/dev-seed-mvp --seller-account-id 1 --validate-derived
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--seller-account-id` | `0` | If set, seed only this seller; do not create a new seller account. |
| `--email` | `demo@example.com` | Demo user when creating accounts. |
| `--password` | `password123` | Bcrypt-hashed for **new** users and, when `--reset-password=true`, for **existing** demo/admin (and seller owner with `--seller-account-id`). |
| `--reset-password` | `true` | If the user row already exists, overwrite `password_hash` with `auth.HashPassword(--password)`. |
| `--admin-email` | `admin@example.com` | Admin user when `--with-admin-user=true`. |
| `--seller-name` | `Demo Ozon Seller` | Seller display name on create. |
| `--anchor-date` | `today` | Dataset **end date** in UTC: `today` (calendar day when you run the seed) or `YYYY-MM-DD`. Seed **never** writes business/metric dates after this day. |
| `--days` | `90` | Inclusive history length ending at anchor: `startDate = anchor - (days-1)` … `anchor` (exactly `days` distinct dates). |
| `--products` | `80` | Number of synthetic SKUs. |
| `--seed` | `20260514` | Base deterministic seed (combined with `seller_account_id` in generators). |
| `--reset` | `true` | Delete this seller’s MVP-shaped rows (including stale alerts/chat/recommendations) before insert. |
| `--with-admin-user` | `true` | Ensure admin user exists. |
| `--validate-only` | `false` | Validate options + DB ping + **seed-level checks** for the seller (`--seller-account-id` required); no writes. See [Validation details](#validation-details). |
| `--validate-alert-generation` | `false` | **`--seller-account-id` required.** Run the production alerts engine (see [Validate alert generation](#validate-alert-generation-details)). |
| `--validate-recommendation-generation` | `false` | **`--seller-account-id` required.** Run **`recommendations.Service.GenerateForAccount`** (OpenAI + validator + DB) like seller **Generate** in the app. Requires **open alerts** and **`OPENAI_API_KEY`**. See [Validate recommendation generation](#validate-recommendation-generation-details). |
| `--validate-derived` | `false` | **`--seller-account-id` required.** Read-only: verify alerts, recommendations, chat, and admin support rows exist after **manual** testing. See [Validate derived](#validate-derived-details). |
| `--validate-pricing-legacy` | `false` | With **`--validate-only`**: fail if deprecated **price_risk** / **overstock** pricing scenarios are missing. Default **off** (active MVP does not require economics alerts or pricing UI). |
| `--reset-derived` | `false` | With **`--validate-alert-generation`**: delete this seller’s **`alerts`** and **`alert_runs`** before the run (also drops `recommendation_alert_links` for those alerts via `ON DELETE CASCADE`). |
| `--as-of-date` | *(empty)* | Optional **`YYYY-MM-DD`** for **`--validate-alert-generation`** or **`--validate-recommendation-generation`**; default is `MAX(daily_account_metrics.metric_date)` for the seller (same anchor strategy as seed validation). |

## What the seed creates (input-shaped data)

- Demo user / seller account (unless `--seller-account-id` is set).
- `ozon_connections`: encrypted demo client/API key + Performance token; `valid` seller API and performance status.
- `products`, `orders`, `sales`, `stocks` (Ozon-like payloads in `raw_attributes`).
- `ad_campaigns`, `ad_metrics_daily`, `ad_campaign_skus`.
- `pricing_constraint_rules` (global_default, category_rule, sku_override) and `sku_effective_constraints` (via resolver recompute).
- `sync_jobs` + `import_jobs` + `sync_cursors` for domains `products`, `orders`, `sales`, `stocks`, `ads` (advertising analytics domain is stored as `ads`).
- `seller_billing_state` internal placeholder (not a full billing product implementation).
- **Analytics:** after commit, rebuilds `daily_account_metrics` / `daily_sku_metrics` from source data, then seeds `product_analytics_daily` (account funnel for dashboard **sales_funnel**).
- **Product images:** `primary_image_url` and `image_urls` (stable `picsum.photos` placeholders per category/SKU).

## Validation details

After a successful seed (or any time you want to confirm a seller still has MVP-shaped data):

```bash
cd backend
go run ./cmd/dev-seed-mvp --seller-account-id 1 --validate-only --anchor-date today
```

Pass the same `--anchor-date` you used for seed (e.g. `today` or `2026-05-21`) so validation checks for **no metric/source dates after that anchor** and that `MAX(daily_account_metrics.metric_date)` equals the anchor when account metrics exist.

Requirements:

- `DATABASE_URL` / config must reach Postgres (same as seed).
- **`--seller-account-id` is required** for validate-only (exit code `2` if omitted).
- Schema through migration **`000027`** (and earlier) for `incremental_sync` on `sync_jobs` and `seed_created` on `admin_action_logs`.

The tool prints a tab-separated table:

`Component | Expected | Actual | Status`

Checks cover: seller account; Ozon connection (seller API + Performance token/status); canonical counts (products ≥ 50, orders/sales/stocks, **product images**, activity in the last 7 days relative to the anchor); **no business/metric dates after `--anchor-date`**; `MAX(daily_account_metrics.metric_date)` equals the anchor when metrics exist; optional `daily_account_metrics` row count vs `--days` when both flags are set; rebuilt metrics rows; **`product_analytics_daily`** (funnel rows on anchor, dashboard **sales_funnel** can be `available`); advertising shape (campaigns, metrics, SKU links, weak ROAS, spend-without-result, low-stock advertised SKU); **active** pricing data integrity (rules + effective constraints); **legacy** price_risk/overstock scenarios are **INFO only** unless `--validate-pricing-legacy`; sync jobs, import domains (including `ads` or `advertising`), at least one failed import, sync cursors, billing stub.

**Exit codes:** `0` — all seed-level checks passed; `1` — one or more checks failed; `2` — bad flags or missing `--seller-account-id` for validate-only.

Optional product flows (recommendations, chat) are **not** validated; hints are printed after the table.

## Validate alert generation details

Use after **`make seed-mvp`** (or equivalent) so `daily_account_metrics` / `daily_sku_metrics` exist.

- **Engine:** `internal/alerts.Service.RunForAccountWithType` with `alerts.RunTypeManual` — same orchestration as **`POST /api/v1/alerts/run`** when the client sends `run_type` `manual` (or omits it, defaulting to manual).
- **As-of date:** UTC midnight of **`--as-of-date`** if set; otherwise **`MAX(metric_date)`** from `daily_account_metrics` for the seller (fallback: max `sales.sale_date`, then today), matching the spirit of the UI defaulting to “today” while staying aligned with seeded metric ranges.
- **`--reset-derived`:** optional clean slate for **`alerts`** and **`alert_runs`** for that seller only (does not touch commerce/ads/pricing source rows).

The CLI prints **`Alert group | Count | Status`** for:

| Row | Requirement |
|-----|-------------|
| `alerts_total` | Total rows in **`alerts`** for the seller after the run (`> 0`). |
| `open_alerts` | Open alerts (`> 0`). |
| `open_sales` | At least one open alert in group **`sales`**. |
| `open_stock` | At least one open alert in group **`stock`**. |
| `open_advertising` | At least one open alert in group **`advertising`**. |
| `open_price_economics` | At least one open alert in group **`price_economics`**. |
| `open_high_or_critical` | At least one open alert with severity **`high`** or **`critical`**. |

On failure, the tool prints **hints** pointing at MVP seed generators and `internal/alerts` rule/threshold code.

**Exit codes:** `0` — all checks passed; `1` — engine error or one or more checks failed; `2` — bad flags or invalid **`--as-of-date`** (or mutually exclusive validate flags).

## Validate recommendation generation details

Prerequisites:

1. MVP seed + analytics (same as alert validation).
2. **Open alerts** for the seller — run **`--validate-alert-generation`** or create alerts from **`/app/alerts`**.
3. **`OPENAI_API_KEY`** set in the environment (same as the API server). If the key is missing, the CLI prints a **controlled message** and exits **`2`** (no panic).

Behavior:

- Calls **`recommendations.Service.GenerateForAccount`**, which uses the same **`ServiceConfig`**, context limits, OpenAI client, and validator wiring as **`cmd/api`**.
- **`--as-of-date`**: optional UTC calendar day; default anchor matches **`--validate-alert-generation`** (`MAX(daily_account_metrics.metric_date)` with fallbacks).

After a successful generate, the tool checks (report: **`Recommendation generation | Expected | Actual | Status`**):

| Check | Meaning |
|-------|---------|
| Open alerts | Prerequisite: **`COUNT(open alerts) > 0`**. |
| OpenAI API key | Non-empty **`OPENAI_API_KEY`**. |
| `recommendation_run` | Latest run row exists; terminal **`completed`** or **`failed`** with `error_message` when the service returns an error. |
| `GenerateForAccount` | No error on success path; run id matches summary. |
| `recommendations_total` | **`COUNT(recommendations)`** for seller **> 0**. |
| Open recommendations | **`COUNT` where `status = open`** **> 0**. |
| Links to alerts | **`COUNT(recommendation_alert_links)`** for seller **> 0**. |
| Payloads | At least one row with **non-empty** `supporting_metrics_payload` and `constraints_payload` (not `{}`). |
| `raw_ai_response` | Rows with **non-null** non-empty `raw_ai_response` (admin/raw-AI storage). |
| Public JSON | Sampled list rows are marshaled with the same shape as seller APIs (**no `raw_ai_response` key**), matching **`handlers.MapPublicRecommendationJSON`**. |

**Exit codes:** `0` — all checks passed; `1` — missing alerts, generation/validation failure, or assertion failure; `2` — missing OpenAI key, invalid **`--as-of-date`**, or mutually exclusive validate flags.

## Validate derived details

**`--validate-derived`** uses the same **`Component | Expected | Actual | Status`** table as **`--validate-only`**. It only runs **SELECT**-style counts; **no OpenAI**, **no engines**, **no inserts**.

Checks (all scoped to **`--seller-account-id`**):

- **Alerts:** `alert_runs` **> 0**; open alerts **> 0**; open alerts in groups **sales**, **stock**, **advertising**, **price_economics** each **> 0**.
- **Recommendations:** `recommendation_runs` **> 0**; `recommendations` **> 0**; `recommendation_alert_links` **> 0**; open recommendations **> 0**.
- **Chat:** `chat_sessions`, `chat_messages`, `chat_traces` each **> 0**; at least one **`chat_traces`** row with **`status = completed`**.
- **Admin / support:** row counts for **`recommendation_runs`** and **`chat_traces`** (same tables the admin UI lists); **`admin_action_logs`** for the seller **> 0** (e.g. after admin actions such as viewing raw AI).

**Exit codes:** `0` — all checks passed; `1` — any check failed (manual MVP flow not fully exercised); `2` — bad flags (e.g. mutually exclusive validate modes or missing **`--seller-account-id`**).

## External services

- Does **not** call Ozon APIs.
- **`--validate-recommendation-generation`** calls **OpenAI** using **`OPENAI_API_KEY`** (same as the API). **`--validate-derived`**, **`--validate-only`**, and **`--validate-alert-generation`** do **not** call OpenAI. Chat and ad-hoc Recommendations in the UI still need a key when you use those features.

## Idempotency

Use `--reset=true` (default on `make seed-mvp`) for a clean, repeatable dataset for the chosen seller. Other sellers are never modified.

**Passwords:** with default **`--reset-password=true`**, re-running the seed against the same emails (or the same `--seller-account-id`) resets login passwords to `--password`. Use **`--reset-password=false`** only if you intentionally want to keep existing `password_hash` values.
