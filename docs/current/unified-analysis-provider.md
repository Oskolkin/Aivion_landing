# Unified Analysis Provider (Stage 3)

Stage 3 adds **provider orchestration** for the Unified Analysis path: build an Analysis Pack,
upload it to an OpenAI Code Interpreter container, run a technical smoke Response, poll to
completion, and clean up provider resources. This stage does **not** publish Dashboard or Actions.

**Stage 4** (production analyst + Structured Outputs + technical validation →
`publication_ready` / `publication_blocked`) is documented in
[`unified-analysis-analyst.md`](./unified-analysis-analyst.md). Stage 4 reuses
the same provider adapter with `UNIFIED_ANALYSIS_PROMPT_PROFILE=analysis`,
`UNIFIED_ANALYSIS_ANALYSIS_MAX_OUTPUT_TOKENS=32768` (GPT-4.1 output cap),
`UNIFIED_ANALYSIS_PROVIDER_MAX_ATTEMPTS=2`, and migration `000055`
(unique index excludes terminal `publication_*` statuses).

## Prompt / schema identity (canonical LF)

Prompt and schema hashes are computed over **canonical LF** bytes only:

* Runtime normalization (`lineendings.NormalizeLineEndings*`) converts `CRLF` / bare `CR` → `LF` before hashing and before sending bytes to OpenAI.
* Therefore hashes are independent of OS and of Git `core.autocrlf` / worktree checkout EOLs.
* Invariant: bytes used for `prompt_hash` / `SchemaHash` are byte-identical to the prompt text / `text.format.schema` sent to the provider.
* `.gitattributes` (`eol=lf` for prompt `*.txt` and schema `*.json`) is an additional checkout guard; **runtime normalization is the primary guarantee** for archives, old checkouts, and mixed worktrees.

Current production identity:

| Field | Value |
| --- | --- |
| schema | `aivion_unified_analysis_v10` |
| schema hash | `sha256:8c2474226f0de69ec18709df7da9853fa5fb11e00522caf829e270bc3ecc673b` |
| prompt | `unified_analyst_v1_26` |
| prompt hash | `9b0fd12db06cb28acf3505d254b47b73303cdbd6c6b7c876142d58ff807e3308` |

**`tool_choice` differs by profile.** `StartBackgroundResponse` sends
`tool_choice=required` for the smoke profile (unchanged since Stage 3) but
`tool_choice=auto` for the analysis profile, so the model can emit the final
Structured Output message *after* Code Interpreter calls instead of being
forced into another tool call. This came from real evidence: run 116
(`v1_2`) and run 118 (`v1_3`, `tool_choice=required`) both ended with
Code-Interpreter-only output and no final Structured Output message
(`structured_output_missing`); run 127 (`v1_4`, still `tool_choice=required`)
ended `provider_response_incomplete`. Switching the analysis profile to
`tool_choice=auto` (with prompt `unified_analyst_v1_5`) is the fix; Stage 4 is
**ACCEPTED** on runs 128/129/130 — see
[`unified-analysis-analyst.md`](./unified-analysis-analyst.md) for the full
prompt diff audit and acceptance evidence.

Related docs:

- [`unified-analysis-contracts-v1.md`](./unified-analysis-contracts-v1.md) — frozen pack/output contracts
- [`unified-analysis-pack-builder.md`](./unified-analysis-pack-builder.md) — Stage 2 pack builder
- [`unified-analysis-analyst.md`](./unified-analysis-analyst.md) — Stage 4 production analyst
- [`unified-analysis-publication.md`](./unified-analysis-publication.md) — Stage 5 atomic publication

## Architecture

```text
PostgreSQL (ai_pipeline_runs, mode=unified_analysis)
    ↓
unifiedanalysis.Service
    ├── pack.Service.BuildAndStore  → MinIO artifacts bucket
    ├── provider.OpenAIClient       → Containers / Files / Responses API
    └── ArtifactStore               → ai_pipeline_artifacts (debug summaries)
    ↓
asynq worker tasks (start → poll → cleanup)
```

Packages:

| Package | Role |
| --- | --- |
| `backend/internal/unifiedanalysis` | Run lifecycle orchestration |
| `backend/internal/unifiedanalysis/provider` | OpenAI REST adapter (+ `FakeProvider` for tests) |
| `backend/internal/unifiedanalysis/pack` | Analysis Pack build/store (Stage 2) |
| `backend/internal/jobs` | `unified_analysis.*` asynq handlers |

Runs reuse `ai_pipeline_runs` with `mode = unified_analysis`. There is no separate run table.

## Lifecycle

1. **CreateRun** — insert run (`status=created`), enforce one active run per seller+metric_date unless `force`.
2. **StartRun** (idempotent) — pack build → validate → store → create container → upload ZIP → start background Response.
3. **PollRun** — retrieve Response; smoke → `provider_completed`; analysis profile →
   `output_validating` and enqueue `unified_analysis.validate_output` (Stage 4).
4. **ValidateOutput** (Stage 4, analysis only) — extract structured JSON, contract
   validator → `publication_ready` | `publication_blocked`.
5. **CleanupRun** — delete container file + container (when `UNIFIED_ANALYSIS_CLEANUP_ENABLED=true`).
6. **CancelRun** — request cancellation; cancel in-flight Response when present; optionally cleanup.
7. **RecoverAll** / **PlanRecovery** — worker startup scans recoverable runs and
   re-enqueues poll / validate / cleanup tasks.

Worker wiring: `backend/cmd/worker/main.go` registers handlers when `cfg.UnifiedAnalysis.Enabled`.

## Statuses

Run statuses (`mode=unified_analysis`):

| Status | Meaning |
| --- | --- |
| `created` | Run row created |
| `pack_building` / `pack_validating` / `pack_ready` | Pack pipeline |
| `provider_container_creating` … `provider_file_ready` | Container setup |
| `provider_response_starting` / `provider_running` | Background Response in flight |
| `provider_completed` | Smoke succeeded (terminal for smoke profile) |
| `output_validating` | Stage 4: structured output extracted, validator running |
| `publication_ready` / `publication_blocked` | Stage 4 terminal validation outcomes |
| `failed` / `cancelled` | Terminal failure / user cancel |
| `cleanup_pending` / `cleanup_running` / `cleanup_completed` | Reserved cleanup run statuses |

`cleanup_status` (separate column) tracks post-provider cleanup: `pending`, `running`, `completed`, `failed`.

Terminal run statuses: `failed`, `cancelled`, `provider_completed`, `publication_ready`,
`publication_blocked`, `cleanup_completed`.

## Polling

- Worker `unified_analysis.poll` calls `Service.PollRun`.
- In-progress Responses reschedule poll with backoff (`UNIFIED_ANALYSIS_POLL_INTERVAL_SECONDS`, capped steps).
- `UNIFIED_ANALYSIS_MAX_POLL_DURATION_SECONDS` can fail the run with `provider_poll_timeout`.
- **Logical** terminal poll outcomes enqueue `unified_analysis.cleanup`. Intermediate retryable terminals (incomplete/`max_output_tokens`, completed tool-only) stay `provider_running`, do **not** enqueue cleanup, and keep the container/file for the next attempt.

## Provider retry (in-run)

`provider_attempt_count` is the number of OpenAI Responses started for this logical run (increments on each successful `SaveResponseStarted` / `StartNextProviderAttempt`).

Default `UNIFIED_ANALYSIS_PROVIDER_MAX_ATTEMPTS=2` means 1 initial attempt + at most 1 automatic retry.

| Case | Retry? |
| --- | --- |
| `incomplete` + reason `max_output_tokens`, attempt &lt; max, analysis budget not already at model cap exhaustion after a 32768 attempt | yes (same pack/prompt/schema/container/file; `max_output_tokens=32768`; `tool_choice=auto`) |
| `completed` + CI present + no assistant/`output_text` (structured output missing), attempt &lt; max | yes (same resources; `tool_choice=auto`; `max_output_tokens=32768`) |
| `incomplete` + `content_filter` / other non-max-output reasons | no |
| provider refusal | no |
| unknown / non-allowlisted `failed` error codes | no |
| second attempt also tool-only or max-output incomplete at 32768 | fail with full attempt artifacts; no third attempt |

Retry is concurrency-safe: CAS `StartNextProviderAttempt` requires `status=provider_running`, matching expected old `provider_response_id`, and `provider_attempt_count < max`. Losers cancel any orphan Response they created and continue from the winning response id.

Order: persist terminal attempt artifact → create new OpenAI Response → CAS swap response id → on CAS loss cancel orphan. Cleanup is not enqueued between attempts.

## Cleanup

`CleanupRun` uses `provider.CleanupContainer` (delete file, then container; 404 = success).
Cleanup errors are stored in `cleanup_error` without overwriting `provider_completed` status.

Cleanup runs only after a **logical** terminal: structured output accepted into Stage 4 / smoke success, retry budget exhausted, non-retryable provider error, or user cancel.

## Recovery

On worker start (when provider enabled):

1. `PlanRecovery` walks recoverable non-terminal runs.
2. Enqueue `unified_analysis.start|poll|validate_output|cleanup` as needed.

Recoverable: in-progress pack/provider states; analysis `provider_completed` /
`output_validating`; `publication_*` with incomplete cleanup.

After a worker restart mid-retry, recovery continues **polling the current** `provider_response_id`. It does not re-start a previous attempt.

## Configuration

Loaded via `unifiedanalysis.ConfigFromEnv()` → `cfg.UnifiedAnalysis`.

Token budget priority per profile: profile-specific env → deprecated `UNIFIED_ANALYSIS_MAX_OUTPUT_TOKENS` → profile default.

| Variable | Default | Notes |
| --- | --- | --- |
| `UNIFIED_ANALYSIS_PROVIDER_ENABLED` | `false` | Primary enable flag |
| `UNIFIED_ANALYSIS_ENABLED` | — | Alias for provider enabled |
| `UNIFIED_ANALYSIS_PROVIDER` | `openai` | Only `openai` supported |
| `UNIFIED_ANALYSIS_MODEL` | — | Required when enabled |
| `UNIFIED_ANALYSIS_CODE_INTERPRETER_MEMORY` | `4g` | `1g`, `4g`, `16g`, `64g` |
| `UNIFIED_ANALYSIS_ANALYSIS_MAX_OUTPUT_TOKENS` | `32768` | Analysis profile (GPT-4.1 cap) |
| `UNIFIED_ANALYSIS_SMOKE_MAX_OUTPUT_TOKENS` | `2000` | Smoke profile |
| `UNIFIED_ANALYSIS_MAX_OUTPUT_TOKENS` | — | Deprecated fallback when profile-specific unset |
| `UNIFIED_ANALYSIS_PROVIDER_MAX_ATTEMPTS` | `2` | In-run provider attempts (1 + retries) |
| `UNIFIED_ANALYSIS_POLL_INTERVAL_SECONDS` | `5` | Poll backoff base |
| `UNIFIED_ANALYSIS_MAX_POLL_DURATION_SECONDS` | `1800` | Provider poll timeout |
| `UNIFIED_ANALYSIS_HTTP_TIMEOUT_SECONDS` | `60` | OpenAI HTTP client |
| `UNIFIED_ANALYSIS_TRANSPORT_RETRIES` | `1` | Safe retries |
| `UNIFIED_ANALYSIS_CLEANUP_ENABLED` | `true` | Delete container resources |
| `UNIFIED_ANALYSIS_DEBUG_ARTIFACTS` | `true` | Persist request/response summaries |

Also requires `OPENAI_API_KEY` when enabled (unless `AI_PROVIDER_MODE=fake` for local dev).

**Worker note:** queue mode devtools enqueue tasks, but the worker only registers handlers when
`UNIFIED_ANALYSIS_PROVIDER_ENABLED=true` in the worker environment.

## Database fields (migrations `000054`, `000055`)

Added to `ai_pipeline_runs` (000054):

- Pack: `pack_id`, `pack_hash`, `pack_storage_key`, `pack_archive_sha256`, `pack_compressed_bytes`, `pack_uncompressed_bytes`
- Provider: `provider`, `provider_model`, `provider_container_id`, `provider_container_file_id`, `provider_container_file_path`, `provider_response_id`, `provider_status`, `provider_started_at`, `provider_completed_at`, `provider_latency_ms`, `provider_attempt_count`, `provider_request_id`
- Usage: `input_tokens`, `output_tokens`, `total_tokens`, `code_interpreter_calls`
- Cleanup / cancel: `cancel_requested_at`, `cleanup_status`, `cleanup_started_at`, `cleanup_completed_at`, `cleanup_error`
- Versioning: `pipeline_version`, `prompt_version`

Indexes: one active unified run per seller+metric_date (000055 rebuild excludes
terminal `publication_ready` / `publication_blocked`); unique `provider_response_id`.

## Artifacts

When `UNIFIED_ANALYSIS_DEBUG_ARTIFACTS=true`, `ArtifactStore` writes JSON payloads to `ai_pipeline_artifacts`. Artifacts **append** per attempt (history is listable; latest-by-kind helpers still return the newest row).

| Kind | Content |
| --- | --- |
| `unified_analysis_pack_reference` | pack_id, hash, storage key |
| `unified_provider_request_summary` | effective runtime request: attempt, model, background, container/file ids, tool_choice, max_output_tokens, prompt/schema identity + hashes, pack ids, request_id |
| `unified_provider_response_summary` | per-attempt status, preview, CI calls, attempt metadata |
| `unified_provider_response_raw` | raw terminal Response JSON when ≤ 2MB |
| `unified_provider_usage` | token usage + latency (including incomplete) |
| `unified_provider_terminal_attempt` | full diagnostic per terminal: attempt, response_id, request_id, status, incomplete_reason, errors, refusal, assistant_message_count, output_text_bytes, output_item_types, CI calls, usage, raw_bytes |
| `unified_provider_cleanup` | cleanup result |

Every terminal provider status (`completed`, `incomplete`, `failed`, `cancelled`) persists attempt artifacts **before** the run status changes.

## Devtools

```powershell
cd backend

# Readiness (config + postgres/redis/minio + migration)
go run ./cmd/devtools check-unified-provider-readiness --live=false

# Live OpenAI probe (container + tiny zip + CI response)
go run ./cmd/devtools check-unified-provider-readiness --live=true

# Queue smoke (requires worker with UNIFIED_ANALYSIS_PROVIDER_ENABLED=1)
$env:UNIFIED_ANALYSIS_PROVIDER_ENABLED='1'
go run ./cmd/devtools run-unified-provider-smoke `
  --seller-id=1 --metric-date=2026-07-08 `
  --execution=queue --wait=true --timeout=30m

# Direct smoke (no worker; polls in-process)
go run ./cmd/devtools run-unified-provider-smoke `
  --seller-id=1 --metric-date=2026-07-08 `
  --execution=direct --cleanup=true

# Inspect run
go run ./cmd/devtools inspect-unified-provider-run --run-id=123

# Cancel + cleanup
go run ./cmd/devtools cancel-unified-provider-run --run-id=123 --execution=direct
```

Flags for `run-unified-provider-smoke`:

- `--seller-id`, `--metric-date` (required)
- `--force` — bypass active-run guard
- `--wait` (default `true`), `--timeout` (default `30m`)
- `--cleanup` (default `true`, direct mode)
- `--debug-artifacts` (default `true`)
- `--execution=queue|direct` (default `queue`)

Devtools force-enable provider for the command when `OPENAI_API_KEY` is present (or `AI_PROVIDER_MODE=fake`),
but the worker still needs `UNIFIED_ANALYSIS_PROVIDER_ENABLED=true`.

## Errors

Typed codes (`unifiedanalysis.Error`):

| Code | Typical cause |
| --- | --- |
| `provider_disabled` | Feature flag off / missing provider client |
| `pack_validation_failed` | Pack builder validator failed |
| `provider_failed` | OpenAI API / cleanup error |
| `provider_poll_timeout` | Exceeded max poll duration |
| `smoke_validation_failed` | Missing CI call or pack linkage in output |
| `inconsistent_run_state` | Invalid status transition |
| `active_run_exists` | Duplicate active run |
| `cancellation_failed` | Provider cancel API error |

## Security

- Pack ZIP is treated as **untrusted** input; smoke instructions forbid executing archive content.
- Container resources are ephemeral; cleanup deletes file + container after run.
- Raw provider responses are capped at 2MB in artifacts.
- API keys only via env; never logged in devtool summaries.
- Stage 3 does not expose unified runs to seller-facing APIs.

## Out of scope (Stage 3)

- Unified Analyst production prompt / Dashboard / Actions generation
- Output validation against `aivion_unified_analysis_v1`
- Publication to dashboard or action_items
- Post-sync automatic triggering
- Frontend UI for unified analysis runs
- Billing / usage quotas beyond token persistence

## Tests

```powershell
cd backend
go test ./internal/unifiedanalysis/... ./internal/jobs/ -count=1

# Optional live OpenAI (costs API usage)
$env:RUN_OPENAI_INTEGRATION_TESTS='1'
go test -tags=openai_integration ./internal/unifiedanalysis/provider/ -run TestOpenAIIntegration -count=1
```
