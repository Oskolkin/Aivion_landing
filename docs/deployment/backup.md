# Backup and restore

## Scripts

| Script | Output | Contents |
|--------|--------|----------|
| `deploy/scripts/backup-postgres.sh [label]` | `/opt/aivion/backups/postgres/*.sql.gz` | `pg_dump` via compose exec |
| `deploy/scripts/backup-minio.sh [label]` | `/opt/aivion/backups/minio/<label>-<ts>/` | `mc mirror` of three buckets |
| `deploy/scripts/backup-config.sh [label]` | `/opt/aivion/backups/config/<label>-<ts>/` | Caddyfile, compose, redacted env, secret **filenames** only |

`deploy.sh` invokes a Postgres backup automatically before cutting over.

## Schedule (recommended)

- Postgres: daily + before every deploy
- MinIO: daily or weekly depending on object churn
- Config: after any Caddyfile / compose / runtime.env change

## Postgres restore (outline)

1. Stop API and worker (`compose stop backend-api backend-worker`).
2. Restore into a clean DB or drop/recreate `marketplace_ai` carefully.
3. `gunzip -c backup.sql.gz | docker compose exec -T postgres psql -U aivion -d marketplace_ai`
4. Run migrations only if restoring an older dump that still needs newer schema (usually: restore matching the release).
5. Start services and smoke-test.

Test restores periodically on a non-production clone.

## MinIO restore (outline)

Use `mc mirror` from the backup directory back to `local/<bucket>` on the compose network (same pattern as `backup-minio.sh`, reversed).

## Evidence

Operational proof for stage-2 acceptance should live on the server:

`/opt/aivion/backups/evidence/`

An index for what to capture is in `docs/audits/stage-2-deployment/README.md`. Do not commit dumps or secret-bearing logs to git.
