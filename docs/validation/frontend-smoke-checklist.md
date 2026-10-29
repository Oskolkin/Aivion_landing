# Frontend manual smoke checklist (MVP)

No Playwright/Cypress suite in this repo. Run these steps after backend + frontend are up locally.

## Prerequisites

```bash
docker compose up -d
# backend/.env.local configured (see README)
cd backend && go run ./cmd/api
cd frontend && npm run dev
```

Demo login (after `make seed-mvp`): `demo@example.com` / `password123`.

## Scenarios

| # | Step | Expected |
|---|------|----------|
| 1 | Open `http://localhost:3000/login`, log in as demo seller | Dashboard or connection gate loads; no 500 |
| 2 | Log out / use account without Ozon connection | Ozon connection onboarding screen; app routes blocked by connection guard |
| 3 | Connected seller: open `/app/dashboard` | Partial or ready home; widgets with `error` / `not_enough_data` show message, page does not crash |
| 4 | Open `/app/products` | Product list loads; sparklines do not show fake zeros for missing metric days |
| 5 | Open `/app/recommendations` | List or empty state; no raw AI JSON in UI |
| 6 | Open `/app/chat` | Chat UI loads; unsupported question returns controlled text (not stack trace) |
| 7 | Non-admin user opens `/app/admin` | Forbidden or redirect; no admin data |
| 8 | Admin email user opens `/app/admin` | Admin clients list (if `ADMIN_EMAILS` includes your user) |

Do not use real Ozon or OpenAI production credentials in this checklist.
