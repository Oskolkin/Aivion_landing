# Server bootstrap

Bootstrap a fresh Linux host (Debian/Ubuntu recommended) for Aivion staging.

Automated helper: `deploy/scripts/server-bootstrap.sh` (run as root). Prefer reading this doc and the script before executing.

## Prerequisites

- VPS/VM with ~6 GB+ RAM, public IPv4, DNS A/AAAA for:
  - `staging.aivion.pro`
  - `app.aivion.pro`
  - `aivion.pro` / `www.aivion.pro`
- SSH key access as a sudo-capable user
- GHCR credentials that can pull:
  - `ghcr.io/oskolkin/aivion-backend`
  - `ghcr.io/oskolkin/aivion-worker`
  - `ghcr.io/oskolkin/aivion-frontend`

## Steps (summary)

1. **Packages** — `curl`, `jq`, `ufw`, `fail2ban`, `apache2-utils` (htpasswd), `openssl`, CA certs.
2. **Docker Engine + Compose plugin** — enable and start `docker`.
3. **Deploy user** — e.g. `aivion`, member of `docker` group. Do not run day-to-day deploys as root.
4. **Directories**
   - `/opt/aivion/{repo,shared,releases,backups/{postgres,minio,config,evidence}}`
   - `/etc/aivion/secrets` (mode `0750`, group readable by deploy user)
5. **Swap** — 2 GB swapfile recommended on 6 GB hosts.
6. **UFW** — allow `22`, `80`, `443` only; deny other inbound. Do **not** open Postgres/Redis/MinIO/backend ports.
7. **Clone repo** into `/opt/aivion/repo` (branch with `deploy/`).
8. **Secrets** — fill `/etc/aivion/secrets/*` (see [secrets.md](./secrets.md)). Never commit values.
9. **runtime.env** — copy `deploy/env/runtime.env.example` → `/opt/aivion/shared/runtime.env`, substitute real values; build `DATABASE_URL` from the postgres password file.
10. **GHCR login** as the deploy user.
11. **First deploy** — `export ACME_EMAIL=...` then `deploy/scripts/deploy.sh <IMAGE_SHA>`.
12. **Admin user** — `deploy/scripts/create-admin.sh --email oskolkinm@yandex.ru`.
13. **Evidence** — store operational artifacts under `/opt/aivion/backups/evidence/` (index: `docs/audits/stage-2-deployment/`).

## DNS and TLS

Point domains at the server before the first Caddy start so ACME HTTP-01 can succeed. Set `ACME_EMAIL` to a monitored address.

## Idempotency

Re-running `server-bootstrap.sh` should:

- skip Docker install if already present
- leave existing secret files untouched
- ensure directories and permissions exist
- not blindly `ufw enable` without review (script prints recommendations only)

## After bootstrap

Continue with [configuration.md](./configuration.md) and [deployment.md](./deployment.md).
