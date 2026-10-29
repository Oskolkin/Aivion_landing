# Rollback

## What rollback does

`deploy/scripts/rollback.sh [IMAGE_SHA]` switches **application images** (backend, worker, frontend) to a previous tag and recreates those services + Caddy. It:

- pulls the target SHA
- does **not** re-run migrations
- does **not** reverse SQL
- updates release metadata / `current_sha`

If omitted, `IMAGE_SHA` defaults to `/opt/aivion/releases/previous_sha`.

## Migration warning

If the newer release applied schema migrations that the older binary cannot run against, **image rollback alone is unsafe**. Options:

1. Prefer a **forward fix** deploy, or
2. Restore Postgres from a pre-deploy backup (`backup-postgres.sh` / `/opt/aivion/backups/postgres/`), then roll images back, or
3. Confirm the schema is backward-compatible before proceeding.

The script requires typing `rollback` to continue.

## Procedure

```bash
export ACME_EMAIL=ops@example.com
# Inspect previous SHA
cat /opt/aivion/releases/previous_sha
# Optional: restore DB first if schema requires it
/opt/aivion/repo/deploy/scripts/rollback.sh <PREVIOUS_SHA>
/opt/aivion/repo/deploy/scripts/smoke.sh
```

## Caddy / promotion rollback

If `promote-app.sh` was used, restore the pre-promote Caddyfile backup created next to `deploy/caddy/Caddyfile` (named `Caddyfile.pre-promote.*`) and recreate Caddy:

```bash
cp -a /opt/aivion/repo/deploy/caddy/Caddyfile.pre-promote.TIMESTAMP \
      /opt/aivion/repo/deploy/caddy/Caddyfile
# then: docker compose ... up -d --force-recreate --no-deps caddy
```

## After rollback

- Verify `/health/ready` and staging Basic Auth still work
- Check worker logs for migration/query errors
- Record evidence under `/opt/aivion/backups/evidence/`
