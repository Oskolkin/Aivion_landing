# Stage-2 deployment acceptance checklist

Use this list after the first staging deploy. Store raw evidence on the server under `/opt/aivion/backups/evidence/` (see `docs/audits/stage-2-deployment/README.md`). Do not commit secrets or full DB dumps to git.

## Infrastructure

- [ ] Only host ports `80` and `443` are published (compose + `ss -tlnp` / UFW)
- [ ] Postgres, Redis, MinIO, API, worker, frontend have no public binds
- [ ] Docker logging `max-size=10m`, `max-file=3`
- [ ] Memory limits present; host stable under idle + light load (~6 GB)
- [ ] Swap enabled

## TLS and routing

- [ ] `https://staging.aivion.pro` serves TLS via Caddy/Let's Encrypt
- [ ] Staging requires Basic Auth (401 without credentials)
- [ ] Authenticated staging loads the app shell
- [ ] `https://app.aivion.pro` returns maintenance **503** HTML (pre-promotion)
- [ ] `https://aivion.pro` returns stub HTML (pre-promotion)
- [ ] HSTS present **without** preload

## Application

- [ ] `GET /health/live` and `GET /health/ready` succeed (via smoke / compose exec)
- [ ] Frontend `/health` succeeds
- [ ] Same-origin API works (browser calls relative `/api/...`; no public backend port)
- [ ] `APP_ENV=staging`, `AUTH_PUBLIC_REGISTRATION=false`, `SESSION_COOKIE_SECURE=true`
- [ ] `ADMIN_EMAILS` includes `oskolkinm@yandex.ru`
- [ ] Admin login works after `create-admin.sh`
- [ ] Worker running with `WORKER_CONCURRENCY=1`
- [ ] OpenAI configured via `OPENAI_API_KEY_FILE` (not plaintext env in compose)

## Data

- [ ] Buckets `raw-payloads`, `exports`, `artifacts` exist
- [ ] Migrations applied successfully
- [ ] Pre-deploy Postgres backup artifact exists under `/opt/aivion/backups/postgres/`

## Operability

- [ ] `deploy.sh <SHA>` completes with release metadata under `/opt/aivion/releases/`
- [ ] `smoke.sh` passes
- [ ] `backup-config.sh` produces redacted output only
- [ ] Rollback procedure reviewed ([rollback.md](./rollback.md)); migration warning understood
- [ ] Promotion path documented; **not** executed until product sign-off (`promote-app.sh`)

## Security hygiene

- [ ] No secret values in git
- [ ] `/etc/aivion/secrets` permissions `0750` / files `0640`
- [ ] `/opt/aivion/shared/runtime.env` not world-readable
- [ ] GHCR credentials stored outside the repo
