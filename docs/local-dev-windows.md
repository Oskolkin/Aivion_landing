# Local development (Windows)

One consistent setup so the dashboard uses the **current** frontend code and API on the **same** port.

## Standard ports

| Service | Port | Notes |
|---------|------|--------|
| Frontend (Next.js **dev**) | **3000** | `npm run dev` only |
| Backend API | **18081** | `BACKEND_PORT=18081` |
| Postgres (compose) | 55432 | `docker compose up -d` |

Do **not** mix `npm start` (production bundle) with active UI work — it does not hot-reload source changes.

## Quick start (two terminals)

### Terminal 1 — backend

```powershell
powershell -ExecutionPolicy Bypass -File scripts/dev/run-api.ps1
# alias:
powershell -ExecutionPolicy Bypass -File scripts/dev/start-backend.ps1
```

This launcher:

- loads root `.env` / `.env.local` (no secrets printed);
- verifies `ENCRYPTION_KEY` is exactly 32 bytes;
- listens on **18081** (matches frontend proxy default);
- sets `UNIFIED_ANALYSIS_ROLLOUT_ENABLED=true` for local read of Unified publications;
- keeps `AUTO_RUN` / `AUTO_PUBLISH` **false** by default.

Safe summary example:

```text
API port: 18081
Database configured: true
Unified rollout enabled: true
Unified auto-run enabled: false
Unified auto-publish enabled: false
```

Do **not** run bare `go run ./cmd/api` without env — it may bind `:8080` and fail on encryption key.

Check:

```powershell
curl.exe http://localhost:18081/health/live
curl.exe http://localhost:18081/api/v1/system/build-info
```

Expect `listen_addr": ":18081"` and HTTP 200. API logs include
`unified analysis runtime config` with rollout / auto-run / auto-publish flags.

### Terminal 2 — frontend

```powershell
powershell -ExecutionPolicy Bypass -File scripts/dev/start-frontend.ps1
```

Ensure `frontend/.env.local` has:

```text
BACKEND_PROXY_URL=http://localhost:18081
NEXT_PUBLIC_API_BASE_URL=
```

(Empty `NEXT_PUBLIC_API_BASE_URL` uses same-origin `/api/v1` rewritten to the proxy.)

### Unified read vs auto-run / auto-publish

| Flag | Affects |
|------|---------|
| `UNIFIED_ANALYSIS_ROLLOUT_ENABLED` | Whether Unified source may be selected for **reads** (and auto flows) |
| `analysis_mode=unified` (seller) | Seller opted into Unified |
| `AUTO_RUN` | Creating new runs after sync only — **does not hide** published data |
| `AUTO_PUBLISH` | Auto-publishing future runs only — **does not hide** already published data |

If seller mode is `unified`, rollout is on, and `seller_current_unified_publications` points at a published visible publication, Dashboard/Actions must show that bundle.

Diagnostics headers on Dashboard/Actions responses:

```text
X-Aivion-Analysis-Source
X-Aivion-Publication-Id
X-Aivion-Run-Id
X-Aivion-Metric-Date
X-Aivion-Fallback-Used
```

First time:

```powershell
cd frontend
copy .env.local.example .env.local
```

Check:

- http://localhost:3000/dev/system → **API base URL: http://localhost:18081**
- Login: `demo@example.com` / `password123` (after `make seed-mvp`)
- Dashboard → **AI-анализ воронки**

### Verify new funnel UI is loaded

In DevTools → Elements, search:

```text
funnel-ai-skeleton-v3
```

If missing, you are on a **stale** bundle (`next start`, old `.next`, or wrong port).

## Clean restart

```powershell
powershell -ExecutionPolicy Bypass -File scripts/dev/stop-dev-ports.ps1
```

Then start backend and frontend again. Hard refresh browser: **Ctrl+F5**.

## Debug funnel view-model on live JSON

```powershell
cd frontend
npx tsx lib/funnel-live-debug.tsx ..\backend\tmp\live-verify-dashboard-home-fresh.json
```

## Common mistakes

| Symptom | Cause | Fix |
|---------|--------|-----|
| Old funnel UI | `next start` on :3000 | Stop process; `npm run dev` |
| API errors / empty dashboard | Frontend proxy ≠ API port | Set `BACKEND_PROXY_URL=http://localhost:18081`, restart frontend |
| Legacy “AI dashboard not generated” with Unified pub | Old API / narrative reject | Restart via `run-api.ps1`; check `X-Aivion-Analysis-Source: unified` |
| `encryption key must be exactly 32 bytes` | Bare `go run ./cmd/api` without env | Use `scripts/dev/run-api.ps1` |
| `8081` / `8080` connection failed | Wrong port | API is on **18081** |

## Production-like frontend check

```powershell
cd frontend
npm run build
npm start
```

Requires rebuild after every code change.
