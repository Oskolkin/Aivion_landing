# Prompt №9.5 Traceability Matrix

## Baseline

| Field | Value |
|-------|-------|
| Branch | `codex/canonical-sales-hardening` |
| Baseline SHA | `d843a02` |
| Working tree | modified (uncommitted Prompt 9.5 closure) |
| Latest migration | `000089_prompt93_collection_provenance` |

---

## Blocker A — Redis/Asynq E2E

| | |
|---|---|
| FILES | `integration_env_test.go`, `prompt95_asynq_integration_test.go`, `prompt9_asynq_integration_test.go` |
| FIX | `integrationRedisURL()` defaults to `redis://127.0.0.1:6379/15`; `TestPrompt95AsynqStages1To6V3` asserts V3 seals + duplicate Core idempotency |
| TEST | `TestPrompt95AsynqStages1To6V3` PASS |
| RESULT | **PASS** |

## Blocker B — Optional capability read guard

| | |
|---|---|
| FILES | `profile_execution_v3.go`, `factual_source.go`, `detail_source.go`, `profile_validate_v3.go` |
| FIX | `ProfileBlockDecision` + `DecideBlockExecutionV3`; optional blocks omitted when capability unavailable |
| TEST | `factual_source_capability_test.go` |
| RESULT | **PASS** |

## Blocker C — Account funnel capability key

| | |
|---|---|
| FILES | `factual_source.go` |
| FIX | Capability from Profile `FUNNEL.ACCOUNT` via `ProfileBlockDecision`, not hardcoded `FUNNEL.DAILY_ADDITIVE` |
| RESULT | **PASS** |

## Blocker D — Phantom Profile methods

| | |
|---|---|
| FILES | `factualapi/service.go`, `factual/sales.go`, `factual/promotions.go`, `factual/finance.go`, `factual/supply.go` |
| METHODS | `BatchGetSKUTrendContext`, `BatchGetSKUPromotionSummary`, `GetFinanceAccountBreakdown`, `GetPromotionAccountSummary`, `GetSupplyAccountSummary` |
| TEST | `profile_method_gate_test.go` PASS |
| RESULT | **PASS** |

## Blocker E — Category Detail V3

| | |
|---|---|
| FILES | `source.go`, `detail_source.go`, `detail.go` |
| FIX | `LoadCategoryDetailExtras`, `CategoryDetailExtras`, batch category reads |
| RESULT | **PASS** |

## Blocker F — SKU Detail funnel

| | |
|---|---|
| FILES | `detail_source.go`, `detail.go`, `source.go` |
| FIX | `BatchGetSKUFunnelPeriod` batch load when `sku.funnel` capability executes |
| RESULT | **PASS** |

## Blocker G — Search query wrong period

| | |
|---|---|
| FILES | `detail_source.go` |
| FIX | `ResolveFactFamilyPeriodPair` for `SEARCH.QUERY_DETAILS` |
| RESULT | **PASS** |

## Blocker H — Source-native windows

| | |
|---|---|
| FILES | `windows.go`, `factual_source.go` |
| FIX | `ResolveBlockPeriodsV3` / `periodFromBlock` for finance, funnel, advertising |
| RESULT | **PASS** |

## Blocker I — Traffic/Attribution separation

| | |
|---|---|
| FILES | `profile_registry_v3.go`, `types.go`, `factual_source.go`, `factual/advertising.go` |
| FIX | `campaign.traffic` / `campaign.attribution` blocks; CPC/CPM/ProviderDRR on DTO |
| RESULT | **PASS** |

## Blockers J–L — Campaign exposure/targeting/stock

| | |
|---|---|
| FILES | `campaign_derive.go`, `factual_source.go` |
| FIX | Exposure share derivation; targeting counts; real stock batch (no fabricated zero) |
| RESULT | **PASS** |

## Blocker M — Detail error → empty

| | |
|---|---|
| FILES | `detail_source.go` |
| FIX | Removed `isFactualRecoverableDetailErr`; only `CodePeriodUnsupported` → UNAVAILABLE |
| RESULT | **PASS** |

## Blocker N — PreDiscovery independent seals

| | |
|---|---|
| FILES | `prediscovery.go`, `prompt94_production_integration_test.go`, `runtime/compose.go` |
| FIX | Ranking/selection hash recompute; expected seals from registry not artifact |
| RESULT | **PASS** |

## Blocker O — Materiality zero fallbacks

| | |
|---|---|
| FILES | `materiality/exact.go`, `ranking.go`, `artifacts.go`, `audit.go` |
| FIX | Removed `parseRatOrZero`/`exactOrZero`; empty decimal errors |
| RESULT | **PASS** |

## Blocker P — CallPlan

| | |
|---|---|
| FILES | `profile_execution_v3.go` |
| TEST | `TestV3ProfileCallPlanNonEmpty`, `TestCallPlanDeterministic` |
| RESULT | **PASS** |

## Blocker Q — Active lane method gate

| | |
|---|---|
| FILES | `active_lane_negative_test.go` |
| TEST | `TestActiveLaneManifestFactualMethodsExistOnAPI` PASS |
| RESULT | **PASS** |

## Blocker R — Query count gates

| | |
|---|---|
| MEASURED | Core 500 = 44, Core 2000 = 72, Detail 15/40/15 = 2 |
| TEST | `query_counter_test.go` PASS |
| RESULT | **PASS** |

## Blocker S — Account summaries (no first-page)

| | |
|---|---|
| FILES | `factual_source.go`, `factual/promotions.go`, `factual/supply.go` |
| FIX | `GetPromotionAccountSummary`, `GetSupplyAccountSummary`, `GetFinanceAccountBreakdown` |
| RESULT | **PASS** |

## Blocker T — Docs current

| | |
|---|---|
| FILES | `docs/current/analysis-runtime.md`, `evals-observability.md`, `analysis-contract-bundle.md`, `entity-landscapes.md` |
| RESULT | **PASS** (9.5 status headers updated) |
