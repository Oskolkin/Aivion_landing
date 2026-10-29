# ai-egress (Xray)

Canonical staging/production OpenAI egress sidecar.

## Why Xray

Local HAPP profiles export **Xray JSON**. Approximate conversion to sing-box is forbidden.
Application contract remains `AI_HTTPS_PROXY=http://ai-egress:10809` (HTTP CONNECT only).
`socks5://` is **not** supported by `backend/internal/openaix` (rejected in `ParseProxyURL`).

## Operator config (not in Git)

Install a **fresh** Xray JSON (never reuse disclosed/compromised profiles):

```text
/etc/aivion/secrets/ai-egress-config.json   # root:65532 0640 (readable by distroless nonroot)
```

Required shape:

1. HTTP inbound listening on `0.0.0.0:10809` (internal Docker network only; not published).
2. Outbound = operator VLESS(+Reality) from a single consistent HAPP/Xray profile.
3. Noauth HTTP inbound is acceptable **only** because the port stays on `aivion_internal`.

Use `deploy/scripts/install-ai-egress-config.sh` (never prints contents).

See `config.example.json` for the inbound template (placeholder freedom outbound).
