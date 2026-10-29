# Deployment

## Preconditions

- Bootstrap complete ([server-bootstrap.md](./server-bootstrap.md))
- Secrets and `runtime.env` in place ([secrets.md](./secrets.md), [configuration.md](./configuration.md))
- DNS points at the server; `ACME_EMAIL` exported
- Logged into GHCR as the deploy user
- Image published for the target SHA:
  - `ghcr.io/oskolkin/aivion-backend:<SHA>`
  - `ghcr.io/oskolkin/aivion-worker:<SHA>`
  - `ghcr.io/oskolkin/aivion-frontend:<SHA>`

## Deploy command

```bash
export ACME_EMAIL=ops@example.com
# optional: export AIVION_IMAGE_DIGEST=sha256:...
sudo -u aivion -H bash -lc '/opt/aivion/repo/deploy/scripts/deploy.sh <IMAGE_SHA>'
```

`deploy.sh` will:

1. Acquire `/var/lock/aivion-deploy.lock` (`flock`)
2. Pull the three GHCR images for `<IMAGE_SHA>`
3. Resolve and print (or verify) digests
4. Run a pre-deploy Postgres `pg_dump` backup
5. `up` dependencies → wait healthy → `minio-init`
6. Run `migrations` (one-shot)
7. `up` API, worker, frontend, Caddy
8. Wait for `/health/ready` and frontend `/health`
9. Write release metadata under `/opt/aivion/releases/`
10. Run `smoke.sh`

## Compose entrypoint

```bash
docker compose \
  -f /opt/aivion/repo/deploy/compose/docker-compose.yml \
  --env-file /opt/aivion/shared/runtime.env \
  ...
```

Always pass `AIVION_IMAGE_TAG` and `ACME_EMAIL`.

## Migrations

Migrations use `migrate/migrate` with `DATABASE_URL` from the `database_url` secret and SQL from `backend/migrations` (path overridable via `AIVION_MIGRATIONS_PATH`).

Failed migrations abort the deploy. Do not skip this step.

## First admin

```bash
export AIVION_ADMIN_PASSWORD_FILE=/etc/aivion/secrets/admin_password
/opt/aivion/repo/deploy/scripts/create-admin.sh --email oskolkinm@yandex.ru
```

The email must appear in `ADMIN_EMAILS`. The backend image must include `/app/admin`.

## Promotion (later)

When ready to open production:

```bash
export ACME_EMAIL=ops@example.com
/opt/aivion/repo/deploy/scripts/promote-app.sh
```

This replaces `deploy/caddy/Caddyfile` with `Caddyfile.promoted` (backup retained) and recreates Caddy. Also add `https://app.aivion.pro` to `CORS_ALLOWED_ORIGINS` and recreate API.

## Smoke

```bash
/opt/aivion/repo/deploy/scripts/smoke.sh
SMOKE_PUBLIC=1 /opt/aivion/repo/deploy/scripts/smoke.sh
```

## Release metadata

`/opt/aivion/releases/<SHA>-<timestamp>/metadata.json` plus:

- `current` → symlink to latest release dir
- `current_sha` / `previous_sha` for rollback
