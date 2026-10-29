# Current unified runtime

This document describes the **actual** post-sync runtime in the repository today: worker orchestration, pipeline state, AI provider modes, and how to run it locally.

## High-level flow

```mermaid
flowchart TD
  A[API: Start sync] --> B[sync_jobs pending]
  B --> C[ozon:sync_coordinator]
  C --> D[import_jobs per domain]
  D --> E[products / orders / stocks / ads]
  E --> F[sync_job completed]
  F --> G[ozon:enrichment_p0]
  G --> H{enrichment status}
  H -->|completed / completed_partial| I[post-sync recalc]
  H -->|failed after retries| I
  I --> J[ozon:run_seller_ai_pipeline]
  J --> K[Data Pack]
  K --> L[Global Context]
  L --> M[Dashboard narrative]
  M --> N[Actions AI-first]
  N --> O[published ai_pipeline_runs]
```

Unified mode is enabled when:

- `SELLER_AI_PIPELINE_ENABLED=true`
- `ACTIONS_AI_FIRST=true`
- `LEGACY_AI_POST_SYNC_ENABLED=false`

`aipipeline.ValidateRuntimeModeConfig()` rejects running unified and legacy post-sync together.

## Runtime components

| Component | Role |
|-----------|------|
| `backend-api` | HTTP API, starts sync jobs, serves dashboard/actions |
| `backend-worker` | Asynq consumer, enrichment, metrics recalc, unified AI pipeline |
| `postgres` | Source data, job state, pipeline runs, actions versions |
| `redis` | Asynq queue |
| `minio` | Raw payloads, exports, debug artifacts |

Worker entrypoint: `backend/cmd/worker/main.go`  
Pipeline wiring: `backend/internal/aipipeline/runtime.go`  
Post-sync handler: `backend/internal/jobs/run_seller_ai_pipeline.go`

## State machines

### Sync job (`sync_jobs`)

`pending → running → completed | failed`

Migration 43 enforces one active (`pending`/`running`) sync per seller.

Types: `initial_sync`, `incremental_sync` (`periodic_sync` legacy rows are normalized on migrate).

### Import job (`import_jobs`)

`pending → fetching → importing → completed | failed`  
(with `retrying` allowed after migration 43)

One row per `(sync_job_id, domain)`.

### Ozon enrichment (`ozon_enrichment_sync_runs`)

`running → completed | completed_partial | failed`

- `completed`: no blocking errors
- `completed_partial`: warnings and/or partial imports
- `failed`: terminal failure with no imported enrichment rows

On terminal failure (including Asynq retry exhaustion), post-sync continues on base ingestion data. The Data Pack records a structured limitation `ozon_enrichment_unavailable` from the latest enrichment run.

### Unified AI pipeline (`ai_pipeline_runs`)

`running → completed | completed_partial | failed | context_failed | dashboard_failed | actions_failed | empty`

Published runs set `published_at` (migration 43). Guards in `Runtime.RunPostSync`:

- skip if another run is `running` for seller+metric_date
- daily max runs (`AI_PIPELINE_MAX_RUNS_PER_DAY`, default 3)
- min interval since last publish (`AI_PIPELINE_MIN_INTERVAL`, default 2h)

Stale `running` rows are marked failed on worker startup/recovery (`AI_PIPELINE_RUNNING_TIMEOUT`, default 60m).

## AI provider modes

Controlled by `AI_PROVIDER_MODE`:

| Mode | Behavior |
|------|----------|
| `fake` | Deterministic fake global context, dashboard, and actions providers (local/tests only) |
| `real` | Alias for `openai`; requires non-placeholder `OPENAI_API_KEY` |
| `openai` | Real OpenAI clients; requires non-placeholder `OPENAI_API_KEY` |

Defaults when `AI_PROVIDER_MODE` is **unset**:

- empty `OPENAI_API_KEY` → `fake` (implicit local/offline convenience)
- real key → `openai`

When `AI_PROVIDER_MODE=real` or `openai` explicitly, missing `OPENAI_API_KEY` is a **startup validation error** — no silent fallback to fake.

`fake` is blocked in `production`/`staging` unless `AI_PROVIDER_ALLOW_FAKE_IN_PRODUCTION=1`.

Root `docker-compose.yml` defaults to `AI_PROVIDER_MODE=real` via `${AI_PROVIDER_MODE:-real}`. For offline local Docker, set `AI_PROVIDER_MODE=fake` in a root `.env` file.

See [pilot-ai-rollout.md](./pilot-ai-rollout.md) for client pilot env, verification, and smoke test.

## Data Pack limitations

`aidatapack` builds `data_quality.structured_limitations[]` entries:

```json
{
  "code": "ozon_enrichment_unavailable",
  "scope": ["product_enrichment", "stock_analytics", "advertising"],
  "message": "Ozon P0 enrichment failed: ...",
  "severity": "high"
}
```

String `data_quality.limitations` remains for backward compatibility.

## Docker Compose (local app profile)

```bash
docker compose --profile app up --build
```

Services: `postgres`, `redis`, `minio`, `migrations`, `backend-api`, `backend-worker`, `frontend`.

Key env on worker (defaults from compose; override via host `.env`):

```env
SELLER_AI_PIPELINE_ENABLED=true
ACTIONS_AI_FIRST=true
AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED=true
LEGACY_AI_POST_SYNC_ENABLED=false
AI_PROVIDER_MODE=real
OPENAI_MAX_RETRIES=0
```

For offline local Docker without OpenAI, add to root `.env`: `AI_PROVIDER_MODE=fake`.

API health: `http://localhost:8081/health/live`  
Worker metrics: `http://localhost:9091/metrics` (token unless `METRICS_ALLOW_UNAUTHENTICATED=true` on API)

## Test commands

From `backend/`:

```bash
# Unit tests
go test ./...

# Integration tests (requires Postgres; CI sets TEST_DATABASE_URL)
export TEST_DATABASE_URL='postgres://postgres:postgres@localhost:55432/marketplace_ai?sslmode=disable'
go test -tags=integration -p 1 ./...

# Focused packages
go test ./internal/aidatapack/...
go test ./internal/httpserver/middleware/...
go test -tags=integration ./internal/db/...
go test -tags=integration ./internal/runtimeintegration/...
```

Redis + MinIO are required for full unified runtime integration tests (`internal/runtimeintegration`).

## Related docs

- [current-ai-first-architecture.md](./current-ai-first-architecture.md) — product-level AI-first pipeline
- [actions-ai-first-stage-3-global-context-pipeline.md](./actions-ai-first-stage-3-global-context-pipeline.md) — global context contract
