# Troubleshooting

## Compose / images

| Symptom | Checks |
|---------|--------|
| `AIVION_IMAGE_TAG must be set` | Export tag before `docker compose` / use `deploy.sh` |
| `ACME_EMAIL must be set` | Export ACME contact email |
| Pull denied from GHCR | `docker login ghcr.io`; token scope `read:packages` |
| Digest mismatch | Confirm CI published the SHA; clear bad local tags |

## Health / readiness

| Symptom | Checks |
|---------|--------|
| API never ready | `compose logs backend-api`; config validation (`APP_ENV=staging` requires JWT length, secure cookies, metrics token, admin emails, closed registration) |
| Frontend unhealthy | Backend ready? `BACKEND_PROXY_URL`? `compose logs frontend` |
| Migrations fail | `database_url` secret; network to postgres; SQL pair up/down integrity |
| Worker crash-loop | OpenAI secret mounted? Redis password matches `runtime.env`? |

## TLS / Caddy

| Symptom | Checks |
|---------|--------|
| ACME failures | DNS A record, ports 80/443 open, no other process bound |
| Staging always 401 | Expected without Basic Auth; verify `staging_basic_auth` secret snippet |
| App returns 503 | Pre-promotion maintenance page — use `promote-app.sh` when ready |
| SSE stalls | Confirm Caddyfile has `flush_interval -1` |

## Data plane

| Symptom | Checks |
|---------|--------|
| Redis NOAUTH | `REDIS_PASSWORD` in runtime.env must match secret used by redis command |
| MinIO access denied | `S3_*` in runtime.env vs `minio_root_*` secrets |
| Disk full | Log rotation (`10m`×3); prune old backups; `docker system df` |

## Resources

Host is sized around **6 GB**. If OOM-killed:

- Confirm compose memory limits
- Ensure swap is on
- Keep `WORKER_CONCURRENCY=1`
- Inspect `dmesg` / `docker events`

## Useful commands

```bash
docker compose -f /opt/aivion/repo/deploy/compose/docker-compose.yml \
  --env-file /opt/aivion/shared/runtime.env ps
docker compose ... logs -f --tail=200 backend-api backend-worker caddy
/opt/aivion/repo/deploy/scripts/smoke.sh
```
