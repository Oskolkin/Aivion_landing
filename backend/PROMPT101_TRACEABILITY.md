# Prompt №10.1 Traceability

## Baseline

- branch: `codex/canonical-sales-hardening`
- baseline SHA: `e9893411ac0d5042356b3b072c36a7a14279c3d1`
- Go module: `1.25.6` (`go.mod` not bumped; local toolchain `go1.26.1`)
- latest migration: `000089_prompt93_collection_provenance`
- NEW_MIGRATION = NO (`000080` already has `analysis_ai_attempts` / `analysis_artifacts` / lease + heartbeat columns)

## Architecture authority

NORMATIVE `docs/architecture/v1/*`. DERIVED `docs/current/*`. Document precedence from `docs/architecture/v1/README.md` unchanged.

## Defects

All 15 post-Prompt-10 defects were CONFIRMED in the baseline tree and fixed in this change. See the completion report in chat for per-defect evidence.

## Contract versions

Bumped (current, old names not left as current):

- `discovery_pack_schema_v2`
- `discovery_fact_handle_registry_v2` / `discovery_fact_handle_map_v2`
- `discovery_evidence_menu_policy_v2`
- `discovery_request_contract_v2`
- `discovery_pack_budget_v2`
- `discovery_validation_v2`
- `discovery_grounding_policy_v2`
- `analysis_orchestration_policy_v4`

Unchanged:

- `discovery_output_schema_v1`, `discovery_output_budget_v1`
- `discovery_prompt_v1`, `discovery_model_policy_v1`, `discovery_ai_attempt_policy_v1`
- `evidence_catalog_v1`
- `factual_api_v5`, landscape schema/profile V5, Materiality, Sales maturity
- `BundleVersionV1`

Composite Discovery seals recomputed via existing `PackContractSeal` / `ValidationContractSeal` (pack seal now includes grounding policy v2).

## Runtime frontier

Stages 1–9 `ImplementedInCurrentBuild=true`. Stage 10 `EXPAND_EVIDENCE` remains `false` and is not in `ProductionImplementedStageNames`. Prompt №11 not started.

## Tests (this environment)

- `go test ./internal/analysis/discovery` PASS
- `go test ./internal/analysis/runtime` PASS
- `go test ./internal/analysis/contracts` PASS
- `go test ./internal/analysis/stages` PASS
- `go test ./internal/openaix` PASS
- `go test ./internal/analysis/...` PASS
- foundation `readiness` / `factualapi` / `factual` / `landscape` / `materiality` PASS
- `go vet ./...` PASS
- `go build ./cmd/api` PASS
- `go build ./cmd/worker` PASS
- `go test ./...` FAIL only on unrelated env: `internal/jobs` Redis localhost forbidden; `unifiedanalysis/prompt` Windows `fork/exec ... Access is denied`
- integration (`TEST_DATABASE_URL` / Redis / MinIO): NOT RUN (env unset)
- real provider smoke: NOT RUN (`AI_PROVIDER_MODE` not `real`)

## Query / N+1

Stage 7 grounding uses planned batch reads only (no per-handle SQL). Standard fixture (`testDetail`): 9 Factual API calls, 5 visible factual objects, 5 Fact Handles, pack 4490 bytes, estimated tokens 1937 (full Responses wire).

## Remaining limitations

- Fake-provider 1→9 Postgres/Redis/MinIO not executed here → `CODE_COMPLETE_VERIFY_PENDING`
- Evidence Expansion resolvers (Prompt №11) not implemented
