# Prompt №9.6 Traceability Matrix

## Baseline

| Field | Value |
|-------|-------|
| Branch | `codex/canonical-sales-hardening` |
| BASELINE_SHA | `9a126c05b873d3e73d5429254d6651059f1adf5d` |
| WORKTREE_STATUS | modified (uncommitted Prompt 9.6) |
| LATEST_MIGRATION | `000089_prompt93_collection_provenance` |

Architecture docs were read; normative `docs/architecture/v1` was not mutated.

---

## PART A — Campaign Traffic / Attribution physical split

| | |
|---|---|
| BLOCKER | Campaign profile used `ADVERTISING.TRAFFIC` for attribution; mixed `CampaignPeriodPerformance` DTO |
| CURRENT FILE | `profile_registry_v3.go`, `factualapi/service.go`, `factual/advertising.go`, `factual_source.go` |
| CURRENT BAD CODE | `requiredBlock("campaign.attribution", … "ADVERTISING.TRAFFIC")`; shared `batchCampaignPerf` period |
| WHY WRONG | Attribution is `ADVERTISING.ATTRIBUTION`; mixed Meta/provenance; Traffic period reused |
| TARGET CONTRACT | `factual_api_v4` + `landscape_schema_v4` separate Traffic vs Attribution methods/periods |
| FILES CHANGED | `factualapi/verticals.go`, `service.go`, `factual/advertising_v4.go`, `profile_registry_v4.go`, `factual_source.go`, `factual_source_ads_v4.go` |
| TEST | `TestPrompt96AdvertisingTrafficAttributionUseIndependentPeriods`, `TestPrompt96CalculatedDRRPeriodMismatch` |
| RESULT | PASS (unit); integration measured on TEST_DATABASE_URL |

## PART B — SKU advertising split

| | |
|---|---|
| BLOCKER | `SKUAdsBlock` mixed Traffic+Attribution |
| FILES CHANGED | `types.go`, `advertising_v4.go`, `profile_registry_v4.go`, `factual_source_ads_v4.go` |
| TEST | landscape unit + production E2E |
| RESULT | PASS |

## PART C — Category / Account advertising split

| | |
|---|---|
| BLOCKER | Category Ads mixed families; `curUnits` discarded |
| FILES CHANGED | `types.go`, `factual_source_ads_v4.go` (`rollupCategoryAdsV4`), `account_derive.go`, `profile_registry_v4.go` (`derivedCapabilityBlockV4`) |
| TEST | production E2E account/category advertising split |
| RESULT | PASS |

## PART D — Advertising domain windows

| | |
|---|---|
| BLOCKER | one `advertising` window copied from sales |
| FILES CHANGED | `factual_source.go` `buildDomainWindows` |
| TEST | `TestDomainWindowsCarryFactFamily`, independent-period integration |
| RESULT | PASS |

## PART E — Category Detail real facts

| | |
|---|---|
| BLOCKER | N+1 `GetCategory*` + discarded metrics; fake READY_EMPTY promotions |
| FILES CHANGED | `detail_source.go`, `detail_source_facts_v4.go`, `factual/category_v4.go` |
| TEST | `TestPrompt96NoCategoryDetailNPlusOne`, production E2E |
| RESULT | PASS |

## PART F — SKU Funnel real facts

| | |
|---|---|
| BLOCKER | funnel value discarded after batch |
| FILES CHANGED | `detail_source_facts_v4.go` |
| TEST | production E2E SKU funnel surfaces |
| RESULT | PASS |

## PART G — SupportingContext provenance

| | |
|---|---|
| BLOCKER | `ApplySupportingTruncation` dropped WindowRef/Period; mixed campaign context |
| FILES CHANGED | `source.go`, `types.go`, `detail_source.go`, `detail_source_facts_v4.go` |
| TEST | `TestApplySupportingTruncationPreservesPeriodAndAsOf` |
| RESULT | PASS |

## PART H — Campaign SKU context semantics

| | |
|---|---|
| BLOCKER | claimed PAID_SPEND_DESC while SQL ordered by SKU id; NOT_FOUND → READY_EMPTY |
| FILES CHANGED | `profile_registry_v4.go`, `detail_source.go`, `factual/advertising.go` (`FOUND_EMPTY`) |
| TEST | `TestPrompt96SelectedCampaignNotFoundFails` |
| RESULT | PASS |

## PART I — Campaign targeting direct relationship

| | |
|---|---|
| BLOCKER | used overall SKU ads for campaign observed performance |
| FILES CHANGED | `advertising_v4.go` `BatchGetCampaignTargetingSummary` |
| TEST | production E2E targeting |
| RESULT | PASS |

## PART J — Campaign stock / exposure

| | |
|---|---|
| BLOCKER | unknown stock → READY_EMPTY; known-zero share unavailable |
| FILES CHANGED | `campaign_derive.go` |
| TEST | `TestCampaignStockUnknownNotReadyEmpty`, `TestCampaignStockMixedKnownUnknown`, `TestShareOfAccountKnownZero` |
| RESULT | PASS |

## PART K — Account demand zero / unknown

| | |
|---|---|
| BLOCKER | amount `"0"` counted as positive sales; known zero → missing |
| FILES CHANGED | `account_derive.go` (`classifySKUCurrentDemand`, `MoneyAccumulator`) |
| TEST | `TestClassifySKUCurrentDemandZeroVsUnknown`, `TestMoneyAccumulatorPreservesKnownZero` |
| RESULT | PASS |

## PART L — Category attributed units

| | |
|---|---|
| BLOCKER | unused `curUnits`/`prevUnits` |
| FILES CHANGED | `factual_source_ads_v4.go` `rollupCategoryAdsV4` |
| RESULT | PASS |

## PART M — Primary NOT_FOUND vs FOUND_EMPTY

| | |
|---|---|
| BLOCKER | primary BatchNotFound treated as empty |
| FILES CHANGED | `factualapi/entities.go` `BatchFoundEmpty`; detail loaders; `BatchListCampaignTargetSKUs` |
| TEST | `TestPrompt96SelectedCampaignNotFoundFails` |
| RESULT | PASS |

## PART N — PreDiscovery V4

| | |
|---|---|
| BLOCKER | no supporting provenance / ad period / DRR coherence validators |
| FILES CHANGED | `prediscovery.go`, `prediscovery_v4.go` |
| TEST | `TestPrompt96ProductionContentfulE2EV4` |
| RESULT | PASS |

## PART O — Materiality V1_4 / Lane Registry V2

| | |
|---|---|
| BLOCKER | merged advertising feature paths |
| FILES CHANGED | `materiality/policy.go`, `versions.go`, `lanes.go`, `from_landscape.go`, `active_lane_manifest.go`, `ranking.go` |
| TEST | materiality unit + production E2E |
| RESULT | PASS |

## PART P–Q — Query shape / full V4 scale

| | |
|---|---|
| BLOCKER | old 44/72/2 not full-V4 proof |
| FILES CHANGED | `query_counter_test.go`, `prompt96_scale_seed_test.go` |
| TEST | `TestPrompt96CoreQueryCount500FullV4` (67), `TestPrompt96CoreQueryShape2000FullV4` (97), `TestPrompt96DetailQueryCountMaxBudgetFullV4` (25) |
| RESULT | measured under integration tags |

## PART R — Regression tests

| | |
|---|---|
| FILES | `prompt96_semantics_test.go`, `detail_source_batch_test.go`, `prompt96_scale_seed_test.go`, `prompt96_production_integration_test.go` |
| RESULT | PASS |

## PART S–T — Contract bundle / PreDiscovery expected V4

| | |
|---|---|
| FILES | `contracts.go`, `production_seals.go`, `runtime/inputhash.go`, `runtime/compose.go` |
| RESULT | PASS |

## PART U–V — Production E2E / Redis Asynq V4

| | |
|---|---|
| FILES | `prompt96_production_integration_test.go`, `prompt96_asynq_integration_test.go` |
| TEST | `TestPrompt96ProductionContentfulE2EV4`, `TestPrompt96AsynqStages1To6V4` |
| RESULT | PASS when TEST_DATABASE_URL / TEST_REDIS_URL / FACTUAL_TOKEN_HMAC_SECRET set |

## PART W — Static gates

| | |
|---|---|
| FILES | `prompt96_static_gate_test.go`, `n_plus_one_test.go`, `profile_method_gate_test.go` |
| RESULT | PASS |

## PART X — Documentation

| | |
|---|---|
| FILES | `docs/current/factual-api.md`, `entity-landscapes.md`, `materiality-engine.md`, `analysis-runtime.md`, `analysis-contract-bundle.md`, `evals-observability.md` |
| RESULT | PASS |

## Post-audit repairs (integration gates)

| BLOCKER | CURRENT FILE | WHY WRONG | FILES CHANGED | TEST | RESULT |
|---|---|---|---|---|---|
| Category `AD_PAID_SPEND_CHANGE` inactive | `factual_source_ads_v4.go` `rollupCategoryAdsV4` | `trafficCur.Add \|\| trafficPrev.Add` short-circuit dropped previous spend | `factual_source_ads_v4.go`, `prompt96_semantics_test.go` | `TestRollupCategoryAdsV4AccumulatesPreviousSpend`, `TestPrompt93AllActiveLanesAndPreDiscovery` | PASS |
| Mem Core emitted V3 `category.advertising` | `category_derive.go`, `profile_validate.go` | Empty wrapper without Traffic/Attribution + derive ignored capability | `category_derive.go` | `TestPrompt9LandscapeMaterialityMemPath` | PASS |
| Prompt 9 Asynq used Prompt 8 seed | `prompt9_asynq_integration_test.go` | `prompt81Seed` lacks V4 required `SALES.ACCOUNT_DEMAND` binding | switch to `prompt91Seed` | `TestPrompt9AsynqStages1to6`, `TestPrompt9AsynqRecoveryLandscapeCore`, `TestPrompt95AsynqStages1To6V3` | PASS |
