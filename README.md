# Marketplace AI MVP

Monorepo for an AI operations cabinet for Ozon sellers: connect Ozon, sync data, and surface dashboard state, risks, SKU focus, alerts, AI summaries, recommendations, and chat. Admin/support endpoints cover technical diagnostics.

| Layer | Stack |
|-------|--------|
| Backend | Go (`backend/`) |
| Frontend | Next.js (`frontend/`) |
| Data | PostgreSQL, Redis, S3/MinIO (Docker Compose) |
| Jobs | Asynq worker (`backend/cmd/worker`) |

## Repository structure

```
backend/          Go API, worker, migrations (backend/migrations/), internal packages
frontend/         Next.js app (App Router)
docker-compose.yml   Local PostgreSQL, Redis, MinIO
docs/             Technical notes; **[current AI-first architecture](./docs/current-ai-first-architecture.md)**; **[current baseline](./docs/current-product-and-tech-baseline-after-20-05.md)** (post-20.05)
Makefile          Common dev commands (infra, migrate, seed)
.env.example      Root env template (copy to .env for local)
```

## Requirements

- **Go** `1.25.6` (see `backend/go.mod`; install matching toolchain for `go test`)
- **Node.js** 20+ and **npm** (lockfile: `frontend/package-lock.json`)
- **Docker** and **Docker Compose** for PostgreSQL, Redis, MinIO
- Optional: `migrate` CLI if you apply migrations manually (API also runs migrations on startup)

## Environment

Copy the template into `backend/.env.local` (loaded by `cmd/api` and `cmd/worker` when run from `backend/`):

```bash
cp .env.example backend/.env.local
```

Edit `backend/.env.local` — required vars are validated at startup (`DATABASE_URL`, `REDIS_ADDR`, `S3_*`, `JWT_SECRET`, `ENCRYPTION_KEY`, etc.).

Key variables for local development:

| Variable | Purpose |
|----------|---------|
| `APP_ENV` | `local` (also `test`, `staging`, `production`) |
| `DATABASE_URL` | PostgreSQL connection string |
| `REDIS_ADDR` | Redis for sessions/jobs |
| `S3_*` | MinIO/S3 for raw payloads and artifacts |
| `JWT_SECRET`, `ENCRYPTION_KEY` | Auth and credential encryption |
| `CORS_ALLOWED_ORIGINS` | Comma-separated browser origins (default `http://localhost:3000`) |
| `SESSION_COOKIE_SECURE` | `false` locally; set `true` behind HTTPS in staging/production |
| `NEXT_PUBLIC_API_BASE_URL` | Frontend → backend URL (e.g. `http://localhost:8081`) |
| `ADMIN_EMAILS` | Comma-separated emails for `/api/v1/admin/*` |
| `AI_SUMMARY_MAX_CATEGORIES_PER_RUN` | Max category AI summaries per run (default `10`) |
| `AI_CHAT_MAX_QUESTIONS_PER_DAY` | Chat questions per seller per UTC day (default `50`) |
| `AI_RECOMMENDATION_MAX_RUNS_PER_DAY` | Recommendation generation runs per day (default `3`) |
| `AI_SUMMARY_MAX_RUNS_PER_DAY` | AI daily summary runs per day (default `3`) |
| `AI_MAX_INPUT_TOKENS_PER_REQUEST` | Per-request input token cap (falls back to `AI_MAX_INPUT_TOKENS_APPROX`) |
| `AI_MAX_OUTPUT_TOKENS_PER_REQUEST` | Per-request output token cap (falls back to `AI_MAX_OUTPUT_TOKENS`; use `8000`–`12000` for dashboard narrative JSON) |
| `DASHBOARD_NARRATIVE_MAX_OUTPUT_TOKENS` | Optional override for dashboard narrative OpenAI `max_output_tokens` (default `12000`) |
| `DASHBOARD_NARRATIVE_DEBUG` | Set `1` to expose narrative freshness/debug fields in `GET /api/v1/dashboard/home` |
| `AI_HTTPS_PROXY` | Optional HTTP(S) proxy for backend → OpenAI only (empty = direct) |
| `DISCONNECTED_ACCOUNT_RETENTION_DAYS` | Days before retention cleanup (default `90`) |
| `WORKER_SCHEDULER_ENABLED` | `true` on one worker replica; `false` on others when running multiple workers |

**Ozon** Seller/Performance API credentials are only required for a real Ozon integration (connection flow).

**OpenAI** `OPENAI_API_KEY` is required for AI summaries, recommendations generation, and chat.

Optional **`AI_HTTPS_PROXY`** routes only backend → OpenAI API traffic through an HTTP(S) proxy (see [AI proxy configuration](docs/ai-proxy-configuration.md)). Ozon, PostgreSQL, Redis, MinIO, and frontend ↔ backend calls are unaffected.

Do not commit `.env`, cookies, or real secrets.

## OpenAI proxy

For local development, OpenAI API requests can be routed through a local HTTP proxy without affecting local app traffic or Ozon requests.

Example (HTTP inbound from a typical local VPN/proxy client):

```env
AI_HTTPS_PROXY=http://127.0.0.1:10809
```

This proxy is used **only** by the backend OpenAI client (`chat`, `recommendations`, AI summaries, unified AI pipeline). It is **not** applied to Ozon Seller/Performance API, PostgreSQL, Redis, MinIO/S3, the frontend, or local backend HTTP handlers.

- **Supported schemes:** `http`, `https`
- **Not supported:** `socks5` (use the HTTP inbound, e.g. `127.0.0.1:10809`, not SOCKS `10808`)
- **Empty `AI_HTTPS_PROXY`:** direct OpenAI connection; OpenAI client ignores `HTTP_PROXY` / `HTTPS_PROXY`

Verify connectivity:

```bash
go run ./cmd/devtools check-ai-connectivity --provider=openai --proxy-diagnostics=true
```

See [docs/ai-proxy-configuration.md](docs/ai-proxy-configuration.md) for production notes.

If your system proxy intercepts localhost, use `--noproxy` for local health checks:

```bash
curl --noproxy "*" http://localhost:8081/health/live
curl --noproxy "*" http://localhost:8081/health/ready
```

Do not commit real proxy credentials in `.env`, README, or tests.

## API build diagnostics

Quick process/version diagnostics endpoint:

```bash
curl --noproxy "*" http://localhost:8081/api/v1/system/build-info
```

Example response:

```json
{
  "service": "marketplace-ai-mvp-api",
  "status": "ok",
  "started_at": "2026-05-28T14:46:00Z",
  "uptime_seconds": 123,
  "build": {
    "version": "dev",
    "commit": "unknown",
    "built_at": "unknown"
  },
  "runtime": {
    "go_version": "go1.25.6",
    "environment": "local",
    "listen_addr": ":8081"
  }
}
```

Build metadata can be injected with `-ldflags`:

```bash
go build -ldflags "-X 'github.com/Oskolkin/marketplace-ai-mvp/backend/internal/buildinfo.BuildVersion=dev' -X 'github.com/Oskolkin/marketplace-ai-mvp/backend/internal/buildinfo.BuildCommit=$(git rev-parse --short HEAD)' -X 'github.com/Oskolkin/marketplace-ai-mvp/backend/internal/buildinfo.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
```

## Local development

1. Start infrastructure (from repo root):

```bash
docker compose up -d
```

Or: `make infra-up`

2. Configure `backend/.env.local` (see [Environment](#environment)).

3. Run the API from `backend/` (applies migrations on startup; reads `MIGRATIONS_PATH=./migrations`):

```bash
cd backend
go run ./cmd/api
```

From repo root via Makefile (same as above, `cd backend` first):

```bash
make backend-run
```

On Windows CMD, optional preset env without a file: `make backend-run-db` (sets `APP_ENV`, `BACKEND_PORT=8081`, `DATABASE_URL` for compose Postgres).

Default API port in `.env.example` is `8081` (CI/Makefile). **Recommended local Windows setup** uses API on **18081** and frontend `NEXT_PUBLIC_API_BASE_URL=http://localhost:18081` — see [docs/local-dev-windows.md](docs/local-dev-windows.md).

**Important:** use `npm run dev` for frontend UI work. `npm start` serves a **production** bundle from `.next` and will not show source changes until you run `npm run build` again.

4. Run the background worker (separate terminal, from `backend/`):

```bash
cd backend
go run ./cmd/worker
```

5. Frontend (from `frontend/`):

```bash
cd frontend
npm install
copy .env.local.example .env.local   # Windows; align API URL with your BACKEND_PORT
npm run dev
```

App: `http://localhost:3000`. Check http://localhost:3000/dev/system for the active API base URL.

**Windows scripts:** `scripts/dev/stop-dev-ports.ps1`, `start-backend.ps1` (port 18081), `start-frontend.ps1` (clears `.next`, runs `next dev`).

**Demo seller (optional):** after DB is up, seed MVP data (`make seed-mvp` from repo root) then log in as `demo@example.com` / `password123` (see `Makefile` `SEED_EMAIL` / `SEED_PASSWORD`). Real Ozon credentials are still required for `PUT /api/v1/integrations/ozon/connection` against live APIs.

## Database migrations

SQL migrations live in `backend/migrations/`.

- **Automatic:** `backend/cmd/api` runs migrations on startup via `MIGRATIONS_PATH` (default `./migrations` when run from `backend/`).
- **Manual:** `make migrate-up` / `make migrate-down` (requires `migrate` CLI and `DATABASE_URL`).

## Tests

**Backend** (from `backend/`):

```bash
go test ./...
```

**Frontend:**

```bash
cd frontend && npm run lint
cd frontend && npm run test
cd frontend && npm run build
```

Manual UI smoke: [docs/validation/frontend-smoke-checklist.md](docs/validation/frontend-smoke-checklist.md).

## Billing (MVP)

Billing is **admin-level placeholder** (`seller_billing_state`): plan/status and token counters for support. There is **no payment provider** or full subscription checkout in this repo. Technical AI limits are enforced via env (`AI_*_MAX_*`), not as a commercial billing product.

## Active vs legacy / deprecated

Full baseline: [docs/current-product-and-tech-baseline-after-20-05.md](docs/current-product-and-tech-baseline-after-20-05.md).

### Active MVP (seller-facing)

- Ozon connection: `GET/PUT /api/v1/integrations/ozon/connection`, `POST .../disconnect`
- Dashboard home, products, alerts/signals, AI daily summaries, recommendations, chat
- Campaign-level advertising alert: `ad_weak_campaign_efficiency`
- Stock/sales alerts as implemented in `backend/internal/alerts` active rules

### Legacy / deprecated / future

- **Pricing constraints UI** removed; `/app/pricing` and `/app/pricing-constraints` redirect to Products. Backend `/api/v1/pricing-constraints/*` remains as a future/deprecated module.
- **Price/economics alerts** disabled in active generation (`PriceEconomicsGenerationDisabled`); types kept for historical records. Frontend `price_economics` group is legacy-only.
- **SKU-level advertising alert types** (`ad_spend_without_result`, `ad_budget_on_weak_sku`, etc.) are legacy; active MVP uses campaign-level `ad_weak_campaign_efficiency` only.
- **Dashboard KPI/stock/ads/funnel/risk counts** come from deterministic analytics, not AI summary JSON. AI summaries provide interpretive text and actions (deduped in the action feed).
- **Technical Ozon endpoints** (`/integrations/ozon/` CRUD, `/check`, `/status`, manual `/initial-sync`) are **admin-only**; onboarding uses `PUT /connection` (which can enqueue initial sync internally).
- **Technical sync/status screens** are not seller-facing; use admin/support diagnostics.

## Security notes

- Session cookie `Secure` flag: `SESSION_COOKIE_SECURE`
- CORS: explicit origin list only (no `*` with credentials)
- Do not track local `*.exe`, `cookies.txt`, or `dashboard-home.json` in git (see `.gitignore`)
