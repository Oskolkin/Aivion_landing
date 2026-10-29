# Aivion deployment architecture

## Scope

This document describes the **staging/production** Docker Compose stack under `deploy/`.  
Local development continues to use the repo-root `docker-compose.yml` (DEV only) and must not be overwritten by production files.

## High-level topology

```text
Internet
   │
   ▼
 Caddy (:80/:443)  ── TLS (Let's Encrypt ACME) ── only published host ports
   │
   ▼
 frontend:3000 (Next.js)
   │  same-origin /api/* rewrites via BACKEND_PROXY_URL
   ▼
 backend-api:8081
   │
   ├── postgres:5432
   ├── redis:6379
   ├── minio:9000
   └── (async) backend-worker
```

| Domain | Role (pre-promotion) | Role (post-promotion) |
|--------|----------------------|------------------------|
| `staging.aivion.pro` | Basic Auth + app | Basic Auth + app |
| `app.aivion.pro` | Maintenance 503 HTML | Live app (reverse_proxy) |
| `aivion.pro` / `www` | Stub HTML | 308 → `app.aivion.pro` |
| `*.review.aivion.pro` | — | Temporary Live Seller Review (see [live-seller-review](../runbooks/live-seller-review.md)) |

## Images (GHCR)

| Service | Image |
|---------|--------|
| API | `ghcr.io/oskolkin/aivion-backend` |
| Worker | `ghcr.io/oskolkin/aivion-worker` |
| Frontend | `ghcr.io/oskolkin/aivion-frontend` |

Tags are set via `AIVION_IMAGE_TAG` (required). Digests are recorded and optionally verified by `deploy/scripts/deploy.sh`.

## Network and ports

- Compose network: `aivion_internal` (bridge).
- **Only Caddy** publishes host ports `80` and `443`.
- Postgres, Redis, MinIO, backend, worker, and frontend have **no** host port bindings.
- The network is not marked `internal: true` so the worker can egress to OpenAI; isolation is “no published ports + UFW allow 22/80/443 only”.

## Why Caddy

Caddy terminates TLS and obtains/renews Let's Encrypt certificates automatically (ACME HTTP-01). That avoids host-level certbot timers and keeps TLS config next to the routing rules in `deploy/caddy/Caddyfile`.

## Same-origin API

Browsers talk only to the public hostname. The frontend sets:

- `NEXT_PUBLIC_API_BASE_URL=""` (relative `/api/...`)
- `BACKEND_PROXY_URL=http://backend-api:8081`

Caddy disables response buffering (`flush_interval -1`) and uses long timeouts for SSE / long AI requests.

## Data stores

| Store | Volume | Notes |
|-------|--------|--------|
| Postgres | `aivion_postgres_data` | User `aivion`, DB `marketplace_ai` |
| Redis | `aivion_redis_data` | `requirepass` from Docker secret; AOF; 256mb LRU |
| MinIO | `aivion_minio_data` | Buckets: `raw-payloads`, `exports`, `artifacts` |
| Caddy | `aivion_caddy_data` / `aivion_caddy_config` | Certificates and runtime config |

## Secrets and runtime env

- Secret **files** live on the server at `/etc/aivion/secrets/` (Docker Compose `secrets:`), mode `0600`.
- Non-secret runtime configuration lives in `/opt/aivion/shared/runtime.env` (`env_file`), generated on the server — **never committed** and must not contain passwords/keys.
- Backend/worker load secrets via `*_FILE` mounts (`OPENAI_API_KEY_FILE`, `DATABASE_URL_FILE`, `JWT_SECRET_FILE`, …).

See [secrets.md](./secrets.md) and [configuration.md](./configuration.md).

## Process model

| Service | Role |
|---------|------|
| `caddy` | Edge TLS + routing |
| `frontend` | Next.js UI + API rewrite proxy |
| `backend-api` | HTTP API (`/health/live`, `/health/ready`) |
| `backend-worker` | Asynq worker; `WORKER_CONCURRENCY=1` |
| `migrations` | One-shot `migrate/migrate` using `database_url` secret |
| `minio-init` | Creates buckets idempotently |
| `postgres` / `redis` / `minio` | Data plane |

Resource limits target roughly **≤6 GB RAM** across the stack (worker is the largest consumer).

## Logging

All services use the Docker `json-file` driver with `max-size=10m` and `max-file=3`.

## Host layout

```text
/opt/aivion/
  repo/                 # git checkout
  shared/runtime.env    # gitignored pattern; server-only
  releases/             # deploy metadata + current_sha
  backups/              # postgres / minio / config / evidence
/etc/aivion/secrets/    # Docker secret source files
```

## Related docs

- [server-bootstrap.md](./server-bootstrap.md)
- [deployment.md](./deployment.md)
- [rollback.md](./rollback.md)
- [backup.md](./backup.md)
- [troubleshooting.md](./troubleshooting.md)
- [acceptance.md](./acceptance.md)
