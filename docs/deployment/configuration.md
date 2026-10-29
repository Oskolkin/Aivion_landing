# Configuration

## Two layers

| Layer | Path | Purpose |
|-------|------|---------|
| Docker secrets | `/etc/aivion/secrets/*` | Mounted into containers as files under `/run/secrets/` |
| Runtime env | `/opt/aivion/shared/runtime.env` | `env_file` for `backend-api` / `backend-worker` |

Example template (no real secrets): `deploy/env/runtime.env.example`.

`runtime.env` is **created on the server** during bootstrap. It must never be committed. Treat it like `.env` (see root `.gitignore` patterns for `*.env` / `.env.*` with exceptions only for `*.example`).

## Required staging defaults

| Variable | Staging value | Notes |
|----------|---------------|--------|
| `APP_ENV` | `staging` | Initial environment; switch to `production` when ready |
| `AUTH_PUBLIC_REGISTRATION` | `false` | Required for deployed envs |
| `SESSION_COOKIE_SECURE` | `true` | Required for deployed envs |
| `ADMIN_EMAILS` | `oskolkinm@yandex.ru` | Comma-separated allowlist |
| `WORKER_CONCURRENCY` | `1` | Protects small hosts / OpenAI spend |
| `CORS_ALLOWED_ORIGINS` | `https://staging.aivion.pro` | Add `https://app.aivion.pro` after promotion |
| `METRICS_ALLOW_UNAUTHENTICATED` | `false` | |
| `NEXT_PUBLIC_API_BASE_URL` | empty (compose) | Same-origin via Next rewrites |
| `BACKEND_PROXY_URL` | `http://backend-api:8081` | Set in compose, not browser |
| `LIVE_REVIEW_ENABLED` | `false` (default) | Temporary Live Seller Review; see [live-seller-review](../runbooks/live-seller-review.md). Never put the invite token in runtime.env. |
| `LIVE_REVIEW_ALLOWED_HOST_SUFFIX` | `.review.aivion.pro` | Required when enabled |
| `LIVE_REVIEW_ALLOWED_SELLER_ACCOUNT_ID` | `1` | Must be in `STAGING_DEMO_SELLER_ACCOUNT_IDS` |

## DATABASE_URL generation

On the server (illustrative):

```bash
PASS="$(tr -d '\r\n' </etc/aivion/secrets/postgres_password)"
# URL-encode PASS if it contains special characters.
URL="postgres://aivion:${PASS}@postgres:5432/marketplace_ai?sslmode=disable"
printf '%s\n' "$URL" >/etc/aivion/secrets/database_url
chmod 0640 /etc/aivion/secrets/database_url
# Also set DATABASE_URL=... inside /opt/aivion/shared/runtime.env
```

Migrations read `/run/secrets/database_url`. The API/worker read `DATABASE_URL` from `runtime.env`.

## OpenAI

Production compose sets:

```text
OPENAI_API_KEY_FILE=/run/secrets/openai_api_key
```

Do not put the raw key in `runtime.env` for staging/production. Local DEV may still use `OPENAI_API_KEY` in `.env.local`.

## Image pins

```bash
export AIVION_IMAGE_TAG=<git-sha>
# optional verification:
export AIVION_IMAGE_DIGEST=sha256:...
export ACME_EMAIL=ops@example.com
```

Compose fails fast if `AIVION_IMAGE_TAG` or `ACME_EMAIL` is unset.

## Redis / MinIO / JWT

- Redis password: Docker secret → `redis-server --requirepass`, and matching `REDIS_PASSWORD` in `runtime.env`.
- MinIO root user/password: Docker secrets via `MINIO_*_FILE`; matching `S3_ACCESS_KEY` / `S3_SECRET_KEY` in `runtime.env`.
- `JWT_SECRET` (≥32 chars) and `ENCRYPTION_KEY` (exactly 32 bytes) and `METRICS_AUTH_TOKEN` in `runtime.env`, sourced from the same files under `/etc/aivion/secrets/`.

## Feature flags

Keep AI pipeline flags aligned with the intended pilot (see `runtime.env.example`). Prefer changing flags via `runtime.env` + recreate API/worker, not by baking into images.
