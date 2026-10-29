# Secrets

## Rules

1. **Never commit** secret values, `runtime.env`, htpasswd plaintext, or API keys.
2. Server path for Docker secret **files**: `/etc/aivion/secrets/`.
3. Permissions: directory `0700` root:root; secret files `0600` root:root.
4. Evidence and backups must not include raw secret contents (`backup-config.sh` redacts).
5. Application containers receive secrets via Docker Compose `secrets:` mounts under `/run/secrets/` and `*_FILE` environment variables — not as plaintext in `docker inspect` env when avoidable.

## Secret files

| File | Used by | Runtime contract |
|------|---------|------------------|
| `openai_api_key` | backend-api, backend-worker | `OPENAI_API_KEY_FILE=/run/secrets/openai_api_key` |
| `postgres_password` | postgres | `POSTGRES_PASSWORD_FILE` |
| `redis_password` | redis, backend-api, backend-worker | redis `--requirepass`; `REDIS_PASSWORD_FILE` |
| `minio_root_user` | minio, minio-init, backend | `MINIO_ROOT_USER_FILE`; `S3_ACCESS_KEY_FILE` |
| `minio_root_password` | minio, minio-init, backend | `MINIO_ROOT_PASSWORD_FILE`; `S3_SECRET_KEY_FILE` |
| `jwt_secret` | backend-api, backend-worker | `JWT_SECRET_FILE` (≥32 characters) |
| `encryption_key` | backend-api, backend-worker | `ENCRYPTION_KEY_FILE` (exactly 32 bytes) |
| `metrics_auth_token` | backend-api | `METRICS_AUTH_TOKEN_FILE` |
| `database_url` | migrations, backend-api, backend-worker | `DATABASE_URL_FILE` / migrate secret |
| `staging_basic_auth` | caddy | Caddy snippet with `basic_auth { ... }` |
| `admin_password` | create-admin.sh only | `AIVION_ADMIN_PASSWORD_FILE` (not a Compose secret) |

`runtime.env` must **not** contain passwords, JWT, encryption keys, MinIO credentials, metrics tokens, or OpenAI keys.

## Generating values (examples)

```bash
install -d -m 700 /etc/aivion/secrets
umask 077
openssl rand -base64 48 | tr -d '\n' >/etc/aivion/secrets/postgres_password
openssl rand -base64 48 | tr -d '\n' >/etc/aivion/secrets/redis_password
openssl rand -base64 48 | tr -d '\n' >/etc/aivion/secrets/jwt_secret
# encryption key: exactly 32 bytes
openssl rand -base64 32 | head -c 32 >/etc/aivion/secrets/encryption_key
openssl rand -hex 32 >/etc/aivion/secrets/metrics_auth_token
openssl rand -hex 16 >/etc/aivion/secrets/minio_root_user
openssl rand -base64 32 | tr -d '\n' >/etc/aivion/secrets/minio_root_password
# Build database_url from postgres_password (do not echo the URL into shell history):
python3 - <<'PY'
from pathlib import Path
import urllib.parse
pw = Path('/etc/aivion/secrets/postgres_password').read_text().strip('\r\n')
url = 'postgres://aivion:%s@postgres:5432/marketplace_ai?sslmode=disable' % urllib.parse.quote(pw, safe='')
Path('/etc/aivion/secrets/database_url').write_text(url)
PY
chmod 600 /etc/aivion/secrets/*
```

Do **not** overwrite `/etc/aivion/secrets/openai_api_key` if it already exists.

## Staging Basic Auth (Caddy)

```bash
HASH="$(docker run --rm caddy:2.9-alpine caddy hash-password --plaintext 'choose-a-strong-password')"
printf 'basic_auth {\n\tstaging_user %s\n}\n' "$HASH" >/etc/aivion/secrets/staging_basic_auth
chmod 600 /etc/aivion/secrets/staging_basic_auth
```

The Caddyfile `import`s this file. Do not put plaintext passwords in the Caddyfile.

## OpenAI key rotation

1. Write the new key to a temp file with mode 600.
2. Atomically replace `/etc/aivion/secrets/openai_api_key`.
3. Recreate `backend-api` and `backend-worker`.
4. Run OpenAI smoke without printing the key.
5. Revoke the old key in the OpenAI dashboard when safe.

## Admin password

Prefer interactive `admin create`, or:

```bash
umask 077
# write password once into a file, then:
export AIVION_ADMIN_PASSWORD_FILE=/etc/aivion/secrets/admin_password
/opt/aivion/deploy/scripts/create-admin.sh
shred -u /etc/aivion/secrets/admin_password
```

Never pass the password on the command line or store it in GitHub Actions.

## Rotation (general)

1. Write new secret file(s) with mode 600.
2. Recreate affected services (`docker compose up -d --force-recreate ...`).
3. For Postgres password changes, update the role inside Postgres **before** pointing `database_url` at the new password.

## What stays out of git

- `/opt/aivion/shared/runtime.env`
- `/etc/aivion/secrets/**`
- `/opt/aivion/backups/**`
- Any `.env`, `*.secret`, cookie jars, or PEM keys (covered by root `.gitignore`)
