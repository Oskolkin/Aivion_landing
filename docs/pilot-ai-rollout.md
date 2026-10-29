# Client pilot AI rollout

This runbook describes how to configure the unified AI pipeline for a **client pilot** using the real OpenAI provider. It does not apply to local offline development (use `AI_PROVIDER_MODE=fake` explicitly there).

## Required environment

Set these variables in your deployment secret manager or host `.env` (never commit real keys):

```env
SELLER_AI_PIPELINE_ENABLED=true
ACTIONS_AI_FIRST=true
AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED=true
LEGACY_AI_POST_SYNC_ENABLED=false
AI_PROVIDER_MODE=real
OPENAI_API_KEY=<secret>
OPENAI_MAX_RETRIES=0
AI_MAX_INPUT_TOKENS_PER_REQUEST=120000
AI_MAX_OUTPUT_TOKENS_PER_REQUEST=8192
```

Copy from [`.env.pilot.example`](../.env.pilot.example) as a starting template.

## Secret handling

- `OPENAI_API_KEY` must be injected at deploy time (secret manager, CI/CD vault, or host env).
- Never commit real keys to git, `.env.example`, `docker-compose.yml`, tests, or documentation.
- Placeholder keys (`fake`, `test`, `compose-local-fake-openai-key`, etc.) are rejected when `AI_PROVIDER_MODE=real` or `openai`.

## Compose verification

After setting env, verify resolved compose config:

```bash
docker compose config | grep AI_PROVIDER_MODE
docker compose config | grep SELLER_AI_PIPELINE_ENABLED
docker compose config | grep AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED
docker compose config | grep LEGACY_AI_POST_SYNC_ENABLED
docker compose config | grep OPENAI_MAX_RETRIES
```

PowerShell:

```powershell
docker compose config | Select-String "AI_PROVIDER_MODE|SELLER_AI_PIPELINE_ENABLED|AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED|LEGACY_AI_POST_SYNC_ENABLED|OPENAI_MAX_RETRIES"
```

Expected defaults in root `docker-compose.yml` (when host env is unset):

- `AI_PROVIDER_MODE` → `real`
- `OPENAI_MAX_RETRIES` → `0`
- Unified pipeline flags → enabled
- `LEGACY_AI_POST_SYNC_ENABLED` → `false`
- No hardcoded `AI_PROVIDER_MODE: fake`

## Local offline Docker (not client pilot)

For deterministic local stack without OpenAI, create a root `.env` file:

```env
AI_PROVIDER_MODE=fake
```

Do **not** use fake provider for client pilots.

## Pilot smoke test

1. Start `postgres`, `redis`, `minio`, `backend-api`, `backend-worker`, `frontend` (`docker compose --profile app up --build`).
2. Ensure the seller has a connected/valid Ozon connection (or dev bypass if applicable).
3. Trigger the unified AI pipeline for `seller_id=1` (post-sync or devtools `run-ai-pipeline` with `ALLOW_DEV_AI_PIPELINE=1`).
4. Inspect pipeline diagnostics / usage:
   - Components: `chunk_analysis`, `global_context`, `dashboard_synthesis`, `actions_synthesis`
   - Provider: `openai` (not fake)
   - `retry_count` = 0
   - Usage rows show provider tokens > 0 for real calls
5. Confirm Dashboard published.
6. Confirm Actions published, `completed_partial`, or `empty` as expected for the data.

## Observability checklist

**Diagnostics**

- `shared_chunk_analysis` source generated/artifact
- `chunk_count` > 0
- `validated_signals` > 0 or limitations explain empty
- `validated_candidates` > 0 or Actions empty
- `global_context` status completed
- `dashboard` status completed
- `actions` status completed / completed_partial / empty
- `retry_count` = 0
- Component usage `provider=openai`

**Usage**

- `chunk_analysis` rows exist
- `global_context` row exists (unless artifact-only path)
- `dashboard_synthesis` row exists
- `actions_synthesis` row exists unless zero candidates / no compose
- `attempt=1`
- `request_id` present where provider returned it

**Artifacts**

- `shared_chunk_analysis` always-on
- `global_context` always-on
- Debug artifacts only when debug enabled

## Rollback (without fake provider)

Do not switch client pilots to `AI_PROVIDER_MODE=fake`. Roll back feature flags instead:

```env
AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED=false
```

Or disable the unified path entirely with previously known stable flags, for example:

```env
SELLER_AI_PIPELINE_ENABLED=false
LEGACY_AI_POST_SYNC_ENABLED=true
```

Coordinate rollback with ops; fake provider remains for local/debug only.

## Related docs

- [current-unified-runtime.md](./current-unified-runtime.md) — worker orchestration and provider modes
- [current-ai-first-architecture.md](./current-ai-first-architecture.md) — product-level pipeline
