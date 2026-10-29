# Live Seller Review

Temporary, cookie-independent access for an external AI auditor to browse the real Aivion seller UI as **seller_account_id=1** (staging/public demo data only).

## 1. Purpose

Give a web reader / AI auditor a normal HTTPS URL that opens the live Dashboard, Actions, Chat, Account, and Ozon connection screens with current PostgreSQL/API data — without login, password, cookies, or manual headers.

## 2. Why cookie review is not enough

The existing `/review/{token}` flow redeems an invite into a DB session and sets an HttpOnly cookie. External AI web readers typically cannot store or replay first-party cookies, so SSR `getCurrentUserServer()` redirects to `/login`.

Live Seller Review reuses the same `review_invites` row but authenticates **statelessly** on every request via Caddy-injected internal headers.

## 3. Architecture

```text
browser (no cookie)
  → https://<random>.review.aivion.pro/app/...
  → Caddy (adds X-Aivion-Live-Review* headers; never echoes token)
  → Next.js frontend:3000
      SSR: forwards Live Review headers to BACKEND_INTERNAL_URL
      client: same-origin /api/v1/* → Caddy headers again → Next rewrite → backend
  → backend auth middleware resolves invite (no session row)
  → seller 1 + read_only + review_mode + live_review_mode
  → normal seller handlers / UI
```

Classic cookie review remains available on `https://aivion.pro/review/{token}`.

## 4. DNS prerequisite (one-time)

Point a wildcard at the VPS:

```text
*.review.aivion.pro  →  <VPS public IP>
```

Until DNS exists, Caddy cannot obtain certificates for temporary hosts.

Also ensure on the host:

- `/etc/aivion/live-review.caddy` exists (stub comment when disabled)
- compose mounts it read-only into Caddy
- `LIVE_REVIEW_ENABLED=true` (and related vars) in `/opt/aivion/shared/runtime.env`

Runtime env (no raw token here):

```text
LIVE_REVIEW_ENABLED=true
LIVE_REVIEW_ALLOWED_HOST_SUFFIX=.review.aivion.pro
LIVE_REVIEW_ALLOWED_SELLER_ACCOUNT_ID=1
```

Requires `APP_ENV=staging`, `STAGING_DEMO_MODE=true`, `PUBLIC_DEMO_MODE=true`, and seller `1` in `STAGING_DEMO_SELLER_ACCOUNT_IDS`.

## 5. Create a link

On the VPS:

```bash
sudo /opt/aivion/repo/deploy/scripts/create-live-review.sh 2h
```

TTL may be hours (`2`) or duration (`2h`). Maximum **24 hours**.

Stdout example (token omitted):

```text
status=enabled
expires_at=...
url_file=/etc/aivion/secrets/live_review_url
token_omitted=true
```

## 6. Read the URL

```bash
sudo cat /etc/aivion/secrets/live_review_url
```

Do not commit this file. Do not paste tokens into chat/logs.

State (no token): `/etc/aivion/secrets/live_review_state.json`  
Token for Caddy only: `/etc/aivion/secrets/live_review_token` (mode 600) and inside `/etc/aivion/live-review.caddy`.

## 7. Verify Dashboard

Cold request without cookies:

```bash
curl -sI "$(sudo cat /etc/aivion/secrets/live_review_url)"
# expect 200 (or 307 within /app), no Set-Cookie, no login redirect Location
curl -s "$(sudo cat /etc/aivion/secrets/live_review_url | sed 's#/app/dashboard#/api/v1/auth/me#')"
# expect is_live_review=true, seller_account.id=1, no token fields
```

Open the URL in a private browser window. Expect normal seller UI (no review banner).

Optional navigator: `/live-review-index` (live review only).

## 8. Verify sections

Check:

- `/app/dashboard`
- `/app/actions` + action detail
- `/app/chat` (existing sessions/messages only)
- `/app/account`
- `/app/integrations/ozon` (demo-ready state, no secrets)

## 9. Verify read-only

```bash
HOST="$(python3 -c 'import json; print(json.load(open("/etc/aivion/secrets/live_review_state.json"))["host"])')"
curl -s -o /dev/null -w '%{http_code}\n' -X POST "https://$HOST/api/v1/chat/ask" -H 'Content-Type: application/json' -d '{"question":"x"}'
# expect 403
curl -s -o /dev/null -w '%{http_code}\n' "https://$HOST/api/v1/admin/me"
# expect 403
```

## 10. Revoke

```bash
sudo /opt/aivion/repo/deploy/scripts/revoke-live-review.sh
```

Revokes the invite, replaces Caddy snippet with the disabled stub, reloads Caddy, deletes URL/token/state (token-free audit retained).

## 11. After revoke

```bash
sudo /opt/aivion/repo/deploy/scripts/status-live-review.sh
# enabled=false (or auth_me no longer 200 with seller data)
```

Previous host must stop returning seller data. Ordinary `aivion.pro` sessions are unaffected.

## 12. Security model

| Control | Behavior |
|--------|----------|
| Auth source | Review invite hash in DB + Caddy internal headers |
| Seller | Fixed allow-list seller 1 only (config + invite) |
| Writes | `ReadOnlyGuard` → 403 |
| Admin | `AdminMiddleware` → 403 |
| Main hosts | Spoofed live-review headers stripped |
| Token | Never in Git, NEXT_PUBLIC_*, HTML, JSON, or stdout of create script |
| Cache/SEO | `no-store`, `noindex`, `Referrer-Policy: no-referrer` |

## 13. Known limitations

- Requires wildcard DNS + ACME for each temporary hostname.
- Chat cannot create new AI turns; existing sessions only.
- Buttons may still appear; backend enforces read-only.
- Not a substitute for External Audit Bundle static exports.

## 14. Forbidden uses

Do **not** enable Live Review for real (non-demo) seller cabinets or production client data. Only staging/public demo seller 1.

## 15. Max TTL

**24 hours** (`MaxReviewTTL` / create script).

## 16. Rollback

1. Run `revoke-live-review.sh`.
2. Set `LIVE_REVIEW_ENABLED=false` in runtime.env and recreate `backend-api` if needed.
3. Ensure `/etc/aivion/live-review.caddy` is the disabled stub.
4. `caddy validate` + `caddy reload`.

Classic `/review/{token}` and normal login are independent and remain available.

## Status command

```bash
sudo /opt/aivion/repo/deploy/scripts/status-live-review.sh
```

## Playwright

```bash
cd frontend
LIVE_REVIEW_BASE_URL=https://<host>.review.aivion.pro \
  npx playwright test -c apex-live-review-playwright.config.ts
```
