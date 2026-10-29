# AI proxy configuration

## Principles

1. The backend does **not** contain VPN/VLESS/HAPP configuration.
2. VPN/proxy is run separately:
   - **Local dev:** HAPP/Xray client with an **HTTP** inbound (recommended `127.0.0.1:10809`)
   - **Staging/production RU:** internal Compose service `ai-egress` (**official Xray**) on the Docker network
3. Application code knows only **`AI_HTTPS_PROXY`** — an HTTP(S) proxy URL.
4. **`AI_HTTPS_PROXY` applies only to OpenAI/AI clients** (chat, recommendations, summaries, unified AI pipeline).
5. **Ozon API, PostgreSQL, Redis, MinIO, frontend** always connect directly.
6. **`HTTP_PROXY` / `HTTPS_PROXY` are never used** for AI clients (and must not be set on backend/worker).
7. When `AI_HTTPS_PROXY` is set, OpenAI traffic is **fail-closed**: if the proxy is down, requests fail (no direct OpenAI fallback).

## Why Xray (not sing-box)

HAPP exports **Xray JSON**. Approximate / guessed conversion to sing-box is forbidden.
Do not mix fields from a VLESS URI with a different JSON profile. Disclosed profiles must be rotated and never committed.

## OpenAI client proxy schemes

Resolved only via `AI_HTTPS_PROXY` (then deprecated `AI_PROXY_URL`). Process-wide `HTTP_PROXY` / `HTTPS_PROXY` are ignored.

| Scheme | Supported by `openaix.ParseProxyURL` / `BuildHTTPClient` |
|--------|----------------------------------------------------------|
| `http://` | yes |
| `https://` | yes |
| `socks5://` | **no** (explicit error) |

A HAPP profile that only exposes SOCKS (`127.0.0.1:10808`) is **not** sufficient for Aivion.
The server-side sidecar (and local HAPP for Aivion) must expose an **HTTP** inbound for `AI_HTTPS_PROXY`.

## Configuration

### Local

```env
# Optional. Empty = direct OpenAI connection (regions where OpenAI is reachable).
AI_HTTPS_PROXY=http://127.0.0.1:10809
```

Deprecated alias (local tooling only): `AI_PROXY_URL` — prefer `AI_HTTPS_PROXY`.

### Staging Compose

`deploy/compose/docker-compose.yml` sets for **backend-api** and **backend-worker** only:

```yaml
AI_HTTPS_PROXY: http://ai-egress:10809
```

Operator-provided **Xray** config (not in Git):

```text
/etc/aivion/secrets/ai-egress-config.json   # root:65532 0640 (readable by distroless nonroot)
```

See `deploy/ai-egress/config.example.json` for the required HTTP inbound on `:10809`.
Replace the placeholder `freedom` outbound with a **fresh** VLESS(+Reality) outbound from one consistent HAPP/Xray profile.

Install helper (never prints contents):

```bash
sudo bash deploy/scripts/install-ai-egress-config.sh /path/to/fresh-xray.json
```

## Staging demo data (no live Ozon)

```env
STAGING_DEMO_MODE=true
STAGING_DEMO_SELLER_ACCOUNT_IDS=1
ALLOW_DEV_SEED=1
```

- Allowed only when `APP_ENV=staging`.
- Demo sellers bypass the live Ozon `connection_required` gate.
- Ordinary sellers keep the full Ozon connection gate.
- Seed with `go run ./cmd/devtools prepare-demo-seller-cabinet ...` (requires `ALLOW_DEV_SEED=1` and `STAGING_DEMO_MODE=true` on staging).
- Never enable on `app.aivion.pro` / production.

## Verify connectivity

```bash
go run ./cmd/devtools check-ai-connectivity --provider=openai --proxy-diagnostics=true
```

Success output includes `status: ok`, `latency_ms`, and proxy diagnostics (**host only**, no credentials).

## Security

Do **not** commit:

- VPN config files, VLESS URLs, Reality keys, server addresses, UUIDs, SNI, shortId
- `OPENAI_API_KEY`
- Proxy credentials in logs (backend redacts userinfo in diagnostics)
- Full `AI_HTTPS_PROXY` values with credentials in evidence dumps
