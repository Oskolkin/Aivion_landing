# Prompt №9.8 Traceability Matrix

Pre-Discovery short-history readiness and deterministic semantics hardening. Does **not** implement Prompt №10 Discovery.

## Baseline (before this work)

| Field | Value |
|-------|-------|
| Branch | `codex/canonical-sales-hardening` |
| BASELINE_SHA | `f609db3212253a16ff1557f850cd4246c0a42664` |
| WORKTREE_STATUS | clean |
| LATEST_MIGRATION | `000089_prompt93_collection_provenance` |
| Go module | `go 1.25.6` |

Architecture authority: `docs/architecture/v1` (NORMATIVE) + final Physical Implementation Plan §172 (no 14-day wait). Derived docs under `docs/current/` updated; normative DOCX not rewritten.

## Conflict corrected

| Old code / early DQ docs | Final Physical Plan | Resolution |
|--------------------------|---------------------|------------|
| `salesDays < 14` → `INSUFFICIENT_ANALYSIS_HISTORY` / no Analysis Run | If history insufficient, analyse what is possible with explicit limitations | Candidate-day Core gate only |
| Candidate required 14-day window `[D-13..D]` | Candidate D itself must be mature/usable | Day-local maturity + reconciliation |
| History DQ issues `BLOCK_ANALYSIS_DATE` | Depth is capability/period limitation | `DEGRADE_TO_PARTIAL` |
| `PARTIAL`+`INSUFFICIENT_HISTORY` ineligible | Short history is not an Analysis Date blocker | Typed eligibility |
| Category `finishDemand(days=periodDays)` | Incomplete source period must not be a known full total | Category uses account/source days |
| Landscape dropped `BatchLimited` → `READY_EMPTY` | UNKNOWN never becomes READY_EMPTY | Keep limited Demand; PARTIAL + limitation |

## Policy versions

| Layer | Before | After |
|-------|--------|-------|
| Orchestration | `analysis_orchestration_policy_v1` | `analysis_orchestration_policy_v2` |
| DQ | `dq_policy_v3` | `dq_policy_v4` |
| Capability | `capability_policy_v3` | `capability_policy_v4` |
| Factual API | `factual_api_v4` | `factual_api_v5` |
| Landscape schema / profiles | `landscape_schema_v4` / Profiles V4 | `landscape_schema_v5` / Profiles V5 |
| Sales maturity | `sales_maturity_policy_v2` | unchanged |
| Temporal | `temporal_policy_v1` | unchanged |
| Materiality / lane registry | `MATERIALITY_POLICY_V1_4` / `MATERIALITY_LANE_REGISTRY_V2` | unchanged |
| Bundle structure | `contract_bundle_v1` | unchanged (hash changes with seals) |

## Migration

None. Policy/version columns are TEXT. No new architectural entity required.

## Stage 7

`BUILD_DISCOVERY_PACK` / `DISCOVERY_AI` / `VALIDATE_DISCOVERY` remain `ImplementedInCurrentBuild = false`. Production frontier is stages 1–6 then `IMPLEMENTED_FRONTIER_REACHED`.

## Tests (executed in this session)

| Command | Result |
|---------|--------|
| `go test` focused packages (readiness, factualapi, factual unit, landscape, materiality, contracts, `./internal/analysis/...`) | PASS |
| `go vet ./...` | PASS |
| `go build ./cmd/api` `./cmd/worker` | PASS |
| Prompt 9.7 files via package tests (`prompt97_regression_test.go`, landscape/materiality prompt97) | PASS |
| `go test -tags=integration ./internal/factual ./internal/readiness` after DB restore | PASS |
| `go test -tags=integration -run TestPrompt9{3,4,5,6,7,2}Production… ./internal/analysis/` | PASS |
| Full `go test ./...` | FAIL only `unifiedanalysis/prompt` Windows AV Access denied writing `prompt.test.exe`; unique `-o` PASS |
| Full `go test -tags=integration -p 1 ./...` | NOT a product PASS: shared TEST DB polluted by `internal/db` migrate down/up; unrelated `collection`/`devseed` compile issues |

## Known limitations

- SKU Demand completeness still counts SKU-level daily facts (pre-existing). Landscape now preserves `BatchLimited` instead of mapping it to `READY_EMPTY`.
- ListCategoryDemand still omits categories with no rows; GetCategoryDemand on a complete source period returns known zero for those ids.
- Historical integration fixtures may still seed `dq_policy_v3` / `capability_policy_v3` for replay. Production `CurrentPolicies()` / `contracts.Current()` use v4/v5.
- No live Ozon/OpenAI I/O. No new provider endpoints.
