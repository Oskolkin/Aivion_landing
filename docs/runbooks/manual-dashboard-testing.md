# Manual Dashboard testing (Windows PowerShell)

Acceptance baseline for local manual QA after cleanup:

```text
publication = 92
run = 206
metric_date = 2026-07-08
schema = aivion_unified_analysis_v10
prompt = unified_analyst_v1_29
```

Do **not** run a new OpenAI Unified analysis unless publication 92 is missing.

Final repository branch after cleanup: `integration/dashboard-acceptance`
Cleanup commit: `3c117b47493731659a75a9de693ef356d85a8558`
Confirm tip: `git rev-parse HEAD` and `git merge-base --is-ancestor 3c117b4 HEAD`

---

## 12.1 Requirements

- Docker Desktop (Postgres `55432`, Redis `6379`, MinIO `19000`)
- Go 1.22+ (project uses current toolchain)
- Node.js + npm
- PowerShell 5+/7+
- Optional: `OPENAI_API_KEY` in `.env.local` only if you start a **new** AI run

## 12.2 Git check

```powershell
cd <repo-root>
git status
git rev-parse HEAD
git branch --show-current
```

Expected: clean tree on `integration/dashboard-acceptance` at the handoff SHA.

## 12.3 Infrastructure

From repo root:

```powershell
docker compose up -d
```

Compose file: `docker-compose.yml`.

## 12.4 Migrations

API launcher runs migrations on start. Manual alternative (from `backend/` with env loaded):

```powershell
cd backend
# DATABASE_URL must match .env.local (port 55432)
migrate -path ./migrations -database "$env:DATABASE_URL" up
```

Or simply start API via `scripts/dev/run-api.ps1` (applies migrations).

## 12.5 API

```powershell
powershell -ExecutionPolicy Bypass -File scripts/dev/run-api.ps1
```

Listens on **http://localhost:18081** (launcher forces `BACKEND_PORT=18081`).

## 12.6 Worker

```powershell
powershell -ExecutionPolicy Bypass -File scripts/dev/run-worker.ps1
```

Metrics typically on `9091` or `19091` depending on `.env.local`.

## 12.7 Frontend

```powershell
powershell -ExecutionPolicy Bypass -File scripts/dev/start-frontend.ps1
```

Or:

```powershell
cd frontend
npm install
# ensure BACKEND_PROXY_URL=http://localhost:18081
npm run dev
```

UI: **http://localhost:3000**

## 12.8 Readiness

```powershell
Invoke-WebRequest http://localhost:18081/health/live -UseBasicParsing
Invoke-WebRequest http://localhost:18081/health/ready -UseBasicParsing
# Frontend
Invoke-WebRequest http://localhost:3000 -UseBasicParsing
```

System page: http://localhost:3000/dev/system

## 12.9 Login

Demo seller (seed defaults):

```text
email: demo@example.com
password: password123
```

Login URL: http://localhost:3000/login

## 12.10 Stop (safe)

Stop API/worker/frontend processes (Ctrl+C in their terminals) or:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/dev/stop-dev-ports.ps1
```

Do **not** run `docker compose down -v` (would wipe volumes / publication 92).

Safe infra stop without wiping data:

```powershell
docker compose stop
```

---

# Manual checklist

## Step 1 — Auth

- [ ] Login page opens
- [ ] Auth succeeds
- [ ] No redirect loop
- [ ] No API errors in browser console

## Step 2 — Dashboard

- [ ] Dashboard loads
- [ ] Source = Unified (not legacy fallback)
- [ ] Publication **92** / run **206** (Inspector headers or `x-aivion-publication-id`)
- [ ] Metric date `2026-07-08`

## Step 3 — Seller Pulse

- [ ] Revenue / orders / AOV visible
- [ ] CTR not used instead of AOV
- [ ] Advertising share is store-level (~**26.2%**)
- [ ] No campaign DRR in Seller Pulse

## Step 4 — Health

- [ ] Score + breakdown
- [ ] Risk drivers have human titles (no raw `high_drr_<id>` codes)

## Step 5 — What Changed

- [ ] Block renders; numbers match cards; stable after refresh

## Step 6 — Funnel

- [ ] Stages, conversions, period, AI narrative consistent

## Step 7 — Stock

- [ ] Buckets / risks / coverage; no contradictory status for one SKU

## Step 8 — Advertising

Confirm three distinct semantics:

| Value | Meaning |
| --- | --- |
| ~26.2% | store advertising spend share |
| 53.3% | attributed campaign DRR |
| 80% | policy threshold (not a fact) |

- [ ] Not labeled as one metric
- [ ] Store share в‰  campaign DRR
- [ ] Threshold not shown as observed DRR

## Step 9 — SKU Focus

- [ ] Cohort / sparkline / narrative refer to the same SKUs

## Step 10 — Top Actions

- [ ] Count matches required Actions only (pub 92 may have **0** Top Actions — `no_material_actions`)
- [ ] No padding with recommended Actions

## Step 11 — Actions

- [ ] List/detail open; evidence links coherent (may be empty for pub 92)

## Step 12 — Identity

- [ ] Dashboard and Actions share publication **92** and run **206**

## Step 13 — Refresh Г—3

- [ ] Narrative / Top Actions / SKU Focus / publication unchanged

## Step 14 — Console

- [ ] No console errors / failed API / hydration issues

## Step 15 — Restart smoke

Stop and restart API, worker, frontend **without** deleting volumes.

- [ ] Pointer still publication **92**
- [ ] Dashboard/Actions HTTP 200 and unchanged
