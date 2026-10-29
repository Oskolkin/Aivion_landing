# Analysis Runtime

Status: **Prompt №13.3 CLOSED** — production stages 1–19 (`RESOLVE_CONTEXT` … `POST_PUBLISH`). Frontier: `{last_stage: POST_PUBLISH}`. Acceptance: `backend/PROMPT133_TRACEABILITY.md`.

## Ownership

- PostgreSQL = durable run/stage/attempt/artifact/event state **and** Publication / Action Case SoT
- Redis / Asynq = executor transport only (`analysis.stage.execute`)
- MinIO / MemoryStore = immutable **pre-publish** artifacts
- OpenAI Responses = Stage 8 Discovery and Stage 12 Deep only (stages 14–19: zero AI)

## Composition root

`runtime.NewProductionExecutor` wires handlers 1–19. Stage 18 `PublishAtomic` commits Publication + current pointer + Actions + outbox + PUBLISH success in one transaction (no MinIO in that TX).

Production executor requires `FACTUAL_TOKEN_HMAC_SECRET` (or devtools fallback `FACTUAL_TOKEN_SIGNING_KEY`) with length ≥ 32.

`ANALYSIS_ENABLED=false` (default) means: do **not** auto-schedule / recovery-tick production analysis. It does **not** leave the handler unwired.

## Stages

19 registry keys, all `ImplementedInCurrentBuild`.

1–13 as Prompt №12 (Deep v3 revalidation + continuity evidence links). Stage 10 unions Discovery evidence requests with `CONTINUITY_REQUIRED` requests from previous Action supports (`evidence_request_normalization_v2`).
14. `PLAN_ACTION_RECONCILIATION` — immutable plan
15. `BUILD_PUBLICATION` — candidate artifact
16. `BUILD_PROJECTIONS` — typed projections + row sets (sealed Factual API)
17. `VALIDATE_PUBLICATION`
18. `PUBLISH` — transactional; evaluation class cannot publish
19. `POST_PUBLISH` — outbox; `IMPLEMENTED_FRONTIER_REACHED` `{last_stage: POST_PUBLISH}`

`base_publication_id` is captured at production run INSERT from `seller_current_publications`. Continuity: `CONTINUITY_INITIAL_NO_BASE_V1` or `CONTINUITY_FROM_PUBLICATION_V1`.

See [action-reconciliation.md](action-reconciliation.md), [publication.md](publication.md), [deep-analysis.md](deep-analysis.md).

## Stage input hash

Executable stages must have a real 64-hex `input_hash` **before** claim. Empty / all-zero hashes are rejected (`STAGE_INPUT_HASH_INVALID`). Stage 11 hashes accepted Discovery/Evidence/continuity artifacts + Action Catalog / relationship / pack budget versions. Stage 12 hashes Deep Pack SHA + payload hash + menu/policy/prompt/schema/model/attempt/output-budget versions. Stage 13 hashes accepted Deep output + pack + menu + relationship policy + validation/budget versions.

## Lease fencing

Claim assigns `lease_token` + increments `lease_version`. Heartbeat/success/failure require matching token+version or `LEASE_LOST`. While a claimed handler runs, a bounded heartbeat extends `lease_expires_at`. Heartbeat loss cancels the stage context; the old owner must not finalize success.

## Retry / recovery

Versioned `analysis_orchestration_policy_v7`. Recovery dispatcher scans READY / expired RUNNING / due `FAILED_RETRYABLE`. When `ANALYSIS_ENABLED=true`, worker ticks recovery enqueue. Registry `TimeoutPolicy` is operational (`stage_default` / `ai_default` / `ai_deep` / `async_default`).

## Cancellation

`cancel_requested_at` + reason. At safe boundary: attempt/stage/run → `CANCELLED` with `STAGE_CANCELLED` / `RUN_CANCELLED` events. Normal cancel must **not** land as `FAILED_TERMINAL` / `STAGE_FAILED`.

## NO_OP

Eligible when same seller + universe manifest hash + contract bundle hash + run intent hash match a prior successful production analytical result. FAILED/CANCELLED/STALE are never NO_OP baselines. Contract registry change changes bundle hash → NO_OP false.

