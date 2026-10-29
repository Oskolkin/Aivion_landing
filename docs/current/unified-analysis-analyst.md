# Unified Analysis Analyst (Stage 4)

## Status

Stage 4 implements production Unified Analyst + OpenAI Structured Outputs +
technical output validation through `publication_ready` / `publication_blocked`.

**ACCEPTED** — production prompt `unified_analyst_v1_5` (entity_ids +
limitation-code rules, no invented series, `tool_choice=auto` for the analysis
profile so the final Structured Output message can follow Code Interpreter
calls) is proven via three real OpenAI queue runs (128/129/130). All used
`prompt_version=unified_analyst_v1_5`,
`prompt_hash=58bdc53ab1dd0a5ddd423c4cb9d44d92b31589008be03a3f1b1940c54df42ad3`,
schema `sha256:ae192829…`, model `gpt-4.1-mini`, and reached
`publication_ready` with cleanup completed:

* **Run A (128)** — seller 1 material-actions: `material_actions_found`,
  non-empty `entity_ids` on sku/campaign scopes, `trend_7d` exact match to
  `account_daily.csv` last 7 days.
* **Run B (129)** — eval `scenario_healthy_no_actions`: `no_material_actions`,
  `eval_passed=true`.
* **Run C (130)** — eval `scenario_insufficient_data`: `insufficient_data`,
  limitation codes `account_funnel_unavailable` / `account_metrics_unavailable`,
  `eval_passed=true`.

See [Acceptance evidence](#acceptance-evidence) for full run details and
[Prompt diff audit](#prompt-diff-audit-v1_2-to-v1_5) for why v1_3/v1_4 were
rejected as production candidates. Run **115** (`v1_2`) remains the last proven
`publication_ready` before v1_5 for historical context.

Stage 5 (Dashboard/Actions persistence, publication, frontend, post-sync,
Reviewer, legacy deletion) is **out of scope** for this document. See
[`unified-analysis-publication.md`](./unified-analysis-publication.md) for
Stage 5 publication status.

## Runtime queue flow

```text
CreateRun (status=created)
→ unified_analysis.start
→ pack build/validate/store → container → upload → background Response
→ unified_analysis.poll (until logical terminal; may start a bounded in-run retry)
→ provider_completed | output_validating (analysis profile; only when structured output is present)
→ unified_analysis.validate_output
→ publication_ready | publication_blocked
→ unified_analysis.cleanup (only after logical terminal; not between provider attempts)
```

Smoke profile (`unified_provider_smoke_v1`) still ends at `provider_completed`
and does **not** enter output validation.

Provider completion gate (analysis): after `completed`, refuse → CI/container →
assistant message / non-empty `output_text`. Missing structured output is
classified as `structured_output_missing` and retried once (if budget remains)
**without** entering `output_validating` or cleanup. `ValidateOutput` only
receives responses that already have structured output text.

Direct/devtools mode follows the same service transitions; queue mode requires
a worker with `UNIFIED_ANALYSIS_PROVIDER_ENABLED=true`.

## Prompt

| Field | Value |
| --- | --- |
| profile | `analysis` |
| current version | `unified_analyst_v1_26` (`PromptVersionAnalyst`) |
| package | `backend/internal/unifiedanalysis/prompt/` |
| schema | `aivion_unified_analysis_v10` |
| schema hash (canonical LF) | `sha256:8c2474226f0de69ec18709df7da9853fa5fb11e00522caf829e270bc3ecc673b` |
| model (current target) | `gpt-4.1` |
| max_output_tokens | `32768` (`UNIFIED_ANALYSIS_ANALYSIS_MAX_OUTPUT_TOKENS`) |
| tool_choice (analysis profile) | `auto` |
| provider max attempts | `2` (`UNIFIED_ANALYSIS_PROVIDER_MAX_ATTEMPTS`) |
| prompt hash (canonical LF) | `9b0fd12db06cb28acf3505d254b47b73303cdbd6c6b7c876142d58ff807e3308` |

Hashes are computed after LF line-ending normalization. CRLF checkouts on Windows must not change prompt/schema identity; the same normalized bytes are sent to OpenAI. `.gitattributes` forces LF for prompt/schema embeds as a secondary guard.

### Version history

| Version | Notes |
| --- | --- |
| `unified_analyst_v1` | Initial production analyst; hash `edef506a4be764e2063587fa77ae4ae78a72450fe9c449b17729806f61a84d91` (run 112) |
| `unified_analyst_v1_1` | Runtime pack-identity appendix via `AppendPackIdentityInput` (not in hash) |
| `unified_analyst_v1_2` | Pack-identity + SKU uniqueness hardening; **run 115** `publication_ready` (last proven `publication_ready` before v1_5) |
| `unified_analyst_v1_3` | Added `entity_ids` + limitation-code rules; kept `tool_choice=required` → **rejected**, see [diff audit](#prompt-diff-audit-v1_2-to-v1_5) |
| `unified_analyst_v1_4` | Added "stop tools" wording, still `tool_choice=required` → **rejected**, see [diff audit](#prompt-diff-audit-v1_2-to-v1_5) |
| `unified_analyst_v1_5` | **Current production.** Rebuilt from `v1_2` principles (not from `v1_4`): `entity_ids` + limitation codes, no invented series, concrete steps, compact CI usage, no JSON-in-Python, explicit final-Structured-Output rule; analysis profile switched to `tool_choice=auto` |

### Prompt diff audit: v1_2 to v1_5

| Version | Changes | Risk | Decision |
| --- | --- | --- | --- |
| `v1_2` | Proven `material_actions` path | Empty sku `entity_ids`; invented `trend_7d`; vague steps | Kept as base for `v1_5` |
| `v1_3` | Added `entity_ids` + limitation codes | Longer constraints combined with `tool_choice=required` → model completed via Code Interpreter only, no final Structured Output message | Do not use as production |
| `v1_4` | Added "stop tools" wording | Wording conflicts with `tool_choice=required` (model still forced into a tool call) → incomplete provider response | Do not use as production |
| `v1_5` | `entity_ids`, limitation codes, no invented series, concrete steps, compact CI, no JSON-in-Python, explicit final-Structured-Output rule; analysis `tool_choice=auto` | — | Current production; proven on runs 128/129/130 (see [Status](#status)) |

### Prompt hash

Recompute with:

```powershell
cd backend
go run ./cmd/devtools check-unified-analysis-readiness --live=false
```

Current template hash (`unified_analyst_v1_5`):
`58bdc53ab1dd0a5ddd423c4cb9d44d92b31589008be03a3f1b1940c54df42ad3`.

Hash covers `AnalystV1Instructions + "\n---\n" + AnalystV1Input` only
(`prompt.Hash`). Per-run values appended by `AppendPackIdentityInput`
(`pack_id`, `pack_hash`, `metric_date`) are sent to the provider but
**excluded** from the stored prompt hash.

## Schema

| Field | Value |
| --- | --- |
| name | `aivion_unified_analysis_v1` |
| file | `backend/internal/unifiedanalysis/contract/schema/unified_analysis_output_v1.schema.json` |
| strict | `true` |
| hash | `sha256:ae19282951d65ba1e3fe09fd7e808f5610f618a61bf168be630d64673a26424d` |
| additionalProperties | `false` on all objects |

## Validator boundaries

Technical only (`backend/internal/unifiedanalysis/outputvalidator/`):

* schema / pack / metric identity (`analysis_meta` vs run pack)
* outcome consistency (including `analysis_failed` rules)
* action limits (≤20, top ≤3)
* enums, required texts, local action IDs
* SKU uniqueness across actions
* pack reference existence via `PackReferenceIndex`

Does **not** repair business content or run a second AI pass.

### PackReferenceIndex

Built from the exact Analysis Pack ZIP bytes used by the run:

* `sku_id` column from `sku_summary.csv` (primary SKU index; not legacy `sku`)
* also indexes `campaign_id` from `campaign_summary.csv` and related CSVs
* `limitations.json` codes and `manifest.json` `pack_id`

Fix for duplicate-SKU false positives (run 114): index `sku_id`, not `sku`.

### Entity IDs (`scope.entity_ids`)

* `sku` / `campaign` scope: `entity_ids` length ≥ 1, and every ID must exist in
  `PackReferenceIndex` for that run's pack.
* `account` scope: empty `entity_ids` is allowed.
* Missing required entities → `output_missing_scope_entities`.
* Unknown SKU → `output_unknown_sku`; unknown campaign → `output_unknown_campaign`.
* Same SKU referenced by more than one action → `output_duplicate_sku_across_actions`.
* The OpenAI strict Structured Output subset cannot express "minItems
  conditional on `scope_type`" in JSON Schema, so `entity_ids` stays a
  required array in the schema; the backend validator enforces the
  scope-dependent non-empty rule above at runtime.

### Limitations

Limitation codes must come from `limitations.json` / the fixed allowlist only
(`CodeOutputUnknownLimitation` for anything else); free-text limitation
reasons are invalid.

### Trend (`seller_pulse.trend_7d`)

Computed from `account_daily.csv`, the last 7 available calendar days only.
The model must not invent, smooth, or fill missing days.

### Code Interpreter policy

* Prefer a single consolidated analysis pass; do not reopen files already
  read.
* Never construct the final JSON answer inside Python / Code Interpreter —
  the final answer must be the Structured Output message.
* Stop calling Code Interpreter once analysis is done so the final Structured
  Output message can be emitted.
* Analysis profile Response uses `tool_choice=auto` (required to let the
  final schema message follow Code Interpreter calls); the Stage 3 smoke
  profile is unchanged and still uses `tool_choice=required`.

### `analysis_failed` outcome (frozen)

The AI may return `analysis_outcome=analysis_failed` when the Analysis Pack is
technically present (12 files, valid manifest) but analysis cannot be completed
for documented data-integrity reasons. In that case:

* `actions` must be empty
* `dashboard.top_action_ids` and `recommended_action_ids` must be empty
* `limitations` must contain at least one code

Distinct from provider-layer failures:

| Layer | Examples | Run terminal status |
| --- | --- | --- |
| Provider | refusal, incomplete response, invalid/unparseable JSON | `failed` |
| Contract validator | unknown SKU, meta mismatch, outcome inconsistency | `publication_blocked` |
| Valid AI outcome | `analysis_failed` with empty actions + limitations | `publication_ready` if checks pass |

Semantic scenario checks live in `backend/internal/unifiedanalysis/eval/`.

## Migration 000055

Applied. Rebuilds `uq_ai_pipeline_runs_one_active_unified_analysis` so
`publication_ready` and `publication_blocked` are **excluded** from the
one-active-run unique index (along with `failed`, `cancelled`,
`provider_completed`, `cleanup_completed`).

Output outcome / validation details are stored primarily in
`ai_pipeline_artifacts` (`unified_analysis_output_*` kinds); the run row
carries `status`, `error_code`, pack/provider fields, and `prompt_version`.

## Artifacts

| Kind | Content |
| --- | --- |
| `unified_analysis_output_raw` | Extracted structured JSON |
| `unified_analysis_output_validation` | Validator report |
| `unified_analysis_output_validated` | Canonical validated copy |
| `unified_analysis_output_summary` | Outcome, counts, output hash |

Plus Stage 3 provider artifacts (`unified_provider_*`, pack reference).

## Statuses (analysis profile)

```text
created → … → provider_running → output_validating → publication_ready
                                                  ↘ publication_blocked
```

Provider refusal / incomplete / unparseable JSON → `failed` (not
`publication_blocked`).

## Environment

Requires `OPENAI_API_KEY` when enabled (unless `AI_PROVIDER_MODE=fake`).

| Variable | Default / notes |
| --- | --- |
| `UNIFIED_ANALYSIS_PROVIDER_ENABLED` | `false` |
| `UNIFIED_ANALYSIS_MODEL` | required when enabled |
| `UNIFIED_ANALYSIS_PROMPT_PROFILE` | `analysis` for Stage 4 |
| `UNIFIED_ANALYSIS_PROMPT_VERSION` | `unified_analyst_v1_26` (override optional) |
| `UNIFIED_ANALYSIS_OUTPUT_VALIDATION_ENABLED` | `true` |
| `UNIFIED_ANALYSIS_ANALYSIS_MAX_OUTPUT_TOKENS` | `32768` |
| `UNIFIED_ANALYSIS_SMOKE_MAX_OUTPUT_TOKENS` | `2000` |
| `UNIFIED_ANALYSIS_MAX_OUTPUT_TOKENS` | deprecated fallback |
| `UNIFIED_ANALYSIS_PROVIDER_MAX_ATTEMPTS` | `2` (1 + one automatic retry) |

Worker must register unified handlers when provider is enabled.

## Recovery

Worker `PlanRecovery` may enqueue:

* `unified_analysis.validate_output` for `output_validating` or analysis
  `provider_completed`
* `unified_analysis.cleanup` independently (including after `publication_*`)

## Devtools

```powershell
cd backend

# Offline readiness (prompt/schema hash, migrations, deps)
go run ./cmd/devtools check-unified-analysis-readiness --live=false

# Real seller analysis run (queue; worker required)
go run ./cmd/devtools run-unified-analysis `
  --seller-id=1 --metric-date=2026-07-08 `
  --execution=queue --wait=true --force=true

# Inspect / re-validate a completed run
go run ./cmd/devtools inspect-unified-analysis-output --run-id=<id>
go run ./cmd/devtools validate-unified-analysis-output --run-id=<id>

# Eval scenarios (fictional seller 9042; queue only)
go run ./cmd/devtools run-unified-analysis-evals `
  --scenario=healthy_no_actions `
  --execution=queue --wait=true --timeout=45m --force=true
```

## Acceptance evidence

Real OpenAI runs (seller 1 / `2026-07-08` unless noted):

| Run | Prompt | Result | Notes |
| --- | --- | --- | --- |
| 112 | `unified_analyst_v1` | `publication_blocked` | Invented `analysis_meta` |
| 114 | `unified_analyst_v1_1` | `publication_blocked` | Meta OK; `PackReferenceIndex` was indexing `sku` not `sku_id` (fixed); then duplicate SKUs across Actions |
| **115** | `unified_analyst_v1_2` | **`publication_ready`** | CI=16; tokens 12631/7348; validated hash `65946266…`; quality: stock Actions had empty `entity_ids`. Last proven `publication_ready` before v1_5 |
| 116 | `unified_analyst_v1_2` | `failed` (seller 9042 healthy) | `structured_output_missing` after heavy CI |
| 117 | `unified_analyst_v1_2` | `publication_blocked` (insufficient) | Free-text limitations instead of codes |
| 118 | `unified_analyst_v1_3` | `failed` | Ran with `tool_choice=required`; 20 Code Interpreter calls, model built the final JSON inside Python instead of returning it; no final Structured Output message → `structured_output_missing` |
| 127 | `unified_analyst_v1_4` | `failed` | Added "stop tools" wording but still `tool_choice=required` → `provider_response_incomplete` |
| **128** | **`unified_analyst_v1_5`** | **`publication_ready`** | **Run A (seller):** seller_id=1, metric_date=2026-07-08, pack_id=`pack:1:2026-07-08:4d9ed2e83bac04c4`; provider_status=completed, CI=12, tokens 18425/10993; outcome=`material_actions_found`, actions=3, top=3; validation valid, output_hash=`8f59ac7bdf54b0acff68bf87c08fe7aa83d41450c60fd69e7fc90812b2a51074`; cleanup=completed; `entity_ids` filled for sku and campaign scopes; `trend_7d` exact match to `account_daily.csv` last 7 days |
| **129** | **`unified_analyst_v1_5`** | **`publication_ready`** | **Run B (healthy):** seller 9042, `scenario_healthy_no_actions`; outcome=`no_material_actions`, actions=0; CI=19; cleanup=completed; `eval_passed=true` |
| **130** | **`unified_analyst_v1_5`** | **`publication_ready`** | **Run C (insufficient):** seller 9042, `scenario_insufficient_data`; outcome=`insufficient_data`, actions=0; limitations=`account_funnel_unavailable`, `account_metrics_unavailable` (exact codes); CI=1; cleanup=completed; `eval_passed=true` |

**Stage 4 status: ACCEPTED** — `unified_analyst_v1_5` proven on runs 128/129/130 (A/B/C). Historical context: run 115 (`v1_2`) was the last `publication_ready` before v1_5; runs 116/118/127 document `tool_choice=required` failures that motivated the analysis-profile switch to `tool_choice=auto`.

### Reliability (asynq integration)

| Test | Result |
| --- | --- |
| `TestAsynqAnalysisQueueHappyPath` | PASS |
| `TestAsynqAnalysisRecoveryOutputValidating` | PASS |
| `TestAsynqAnalysisDuplicateValidate` | PASS |
| `TestAsynqQueueHappyPathFakeProvider` | PASS |

## Scope confirmation

Not implemented in Stage 4:

* Dashboard / Actions DB publication
* frontend
* post-sync auto-run
* AI Reviewer / repair
* legacy pipeline removal
