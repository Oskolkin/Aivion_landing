# Prompt №10 Traceability

## Baseline

- branch: `codex/canonical-sales-hardening`
- baseline SHA: `d6cc33bc585e3f9a5ddec430597265de774ffd9d`
- final SHA: not committed
- Go module: `1.25.6` (`go.mod` not bumped)
- latest migration: `000089_prompt93_collection_provenance`
- NEW_DB_MIGRATION = NO (`000080` already has `analysis_ai_attempts` / `analysis_artifacts` / generic runtime)

## Architecture authority

NORMATIVE `docs/architecture/v1/*` (Discovery Pack = canonical JSON; generic analysis runtime tables). DERIVED `docs/current/*`.

## Pre-implementation audit

Existed: stages 1–6, frontier after Detail, `TimeoutPolicy` unused, `AddArtifact` latest-attempt, no Responses client, Discovery contracts `NOT_IMPLEMENTED`.

Missing: stages 7–9, Fact Handle grounding, Evidence Menu, OpenAI adapter, AI attempt persistence, operational `ai_default` timeout, `Work.StageAttemptID`.

## Contract versions

- orchestration: `analysis_orchestration_policy_v2` → `v3`
- pack / fact-handle registry / evidence catalog+menu / pack budget: `discovery_*_v1` composite `DISCOVERY_PACK#…`
- prompt + model policy: `DISCOVERY_PROMPT#…`
- output schema + output budget: `DISCOVERY_OUTPUT#…`
- validation + AI attempt policy (`max_model_attempts=2`): `DISCOVERY_VALIDATION#…`
- SalesMaturity / Materiality / Landscape / Factual API: unchanged
- EvidenceExpansion / Deep: `NOT_IMPLEMENTED`
- `BundleVersionV1` unchanged

## Evidence Catalog

- defined: 50
- enabled for Discovery v1: 34
- disabled: 16 (no honest expansion recipe on current Factual API: account/category sales history & contributors, pricing strategy, category price-change, campaign config, account funnel history, several finance/supply/catalog types)

## OpenAI request

`POST /v1/responses`; model from `OPENAI_DISCOVERY_MODEL` / `OPENAI_MODEL`; `tools: []`; no non-null `previous_response_id`; `store: false`; `truncation: disabled`; `text.format.type=json_schema` `strict=true`; default `max_output_tokens=8192`. Default runtime provider is Fake unless `AI_PROVIDER_MODE=real`.

## Attempt policy

Max 2 model responses; identical request on rerun; no repair prompt; no model fallback. Transport 429/5xx classified for existing runtime retry. Crash after HTTP success before persist is not exactly-once (documented).

## Artifacts

`DISCOVERY_PACK`, `DISCOVERY_FACT_HANDLE_MAP`, `DISCOVERY_REQUEST_MANIFEST`, `DISCOVERY_OUTPUT`, `DISCOVERY_OUTPUT_ATTEMPT`, `DISCOVERY_VALIDATED_OUTPUT`, `DISCOVERY_VALIDATION_REPORT`.

## Tests (this environment)

- `go test ./internal/analysis/discovery` PASS
- `go test ./internal/openaix` PASS
- `go test ./internal/analysis/contracts` PASS
- `go test ./internal/analysis/stages` PASS
- `go test ./internal/analysis/runtime` PASS
- `go test ./internal/analysis/...` PASS
- foundation `readiness` / `factualapi` / `factual` / `landscape` / `materiality` PASS
- `go vet` (focused packages) PASS
- `go build ./cmd/api ./cmd/worker` PASS
- `go test ./...` FAIL only on unrelated env: `internal/jobs` Redis localhost forbidden; `unifiedanalysis/prompt` Windows AV `fork/exec ... being used by another process`. Analysis/discovery/runtime/openaix all PASS.
- integration (`TEST_DATABASE_URL` / Redis / MinIO): NOT RUN (env unset)
- real provider smoke: NOT RUN

## Query / N+1

Stage 7 grounding uses BatchGet* + `ListCategoryDemand` paging, not one query per handle. Additional Stage 7 queries = bounded Factual API reads (account demand/finance/ads periods + SKU/campaign batches + category list pages). Measured integration query count: NOT RUN here.

## Stage 10

`ImplementedInCurrentBuild=false`; not in `ProductionImplementedStageNames`; integration assertions require it not READY/SUCCEEDED.

## Remaining limitations

- Evidence Expansion resolvers (Prompt №11) not implemented; disabled catalog keys wait for honest APIs
- Fake default means worker will not call OpenAI without `AI_PROVIDER_MODE=real`
- Payload-hash vs pack-hash in Stage 8 input hash uses pack output hash as payload stand-in when separate payload artifact is not stored
- No live-AI quality assertions in unit CI
