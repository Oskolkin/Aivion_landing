# Evals / Observability (Prompt №9)

Status: **Prompt №13** — Layer A algorithm golden + Layer B production factual-path suite; Deep Support Basis validation; Publication/Action metrics (`publication_publish_total`, `publication_stale_runs_total`, `publication_transaction_latency`). Observability never repairs Action/Publication data.

## Landscape diagnostics

- build duration / entity counts by kind
- artifact size / global limitation count
- detail selected counts
- validation failures
- bounded DB/query diagnostics where instrumented

## Materiality diagnostics

- eligible entities / candidates per lane / selected counts
- selected sales coverage / ad spend coverage
- unique categories in SKU selection
- lane coverage / redundancy rate
- build duration / artifact size

Coverage metrics are **diagnostic**, not quality PASS/FAIL.

## Golden Materiality suite

Package `internal/analysis/materiality` — `TestGoldenSuiteRecall`:

- Labels: `MUST_INCLUDE` / `SHOULD_INCLUDE` / `OPTIONAL` / `SHOULD_NOT_INCLUDE`
- Gate: critical (`MUST_INCLUDE`) recall **100%**; general material recall **≥95%**
- Prefer recall over precision when tuning pilot defaults

## Production integration (Prompt №9.2)

| Test | Scope |
|------|-------|
| `TestPrompt92ProductionLaneFamiliesStages1to6` | Stages 1–6, all 7 Materiality lane families, non-anchor selection, SKU Campaign detail |
| `TestPrompt92ProvenanceIntegrity` | Source contract hash ≠ mapping hash; DB↔MinIO 3-way integrity; immutability + manifest tamper |
| `TestAnalysisDateMultiFamilyBindings` | LATEST_STABLE emits 3 SEARCH families |

| `TestPrompt96ProductionContentfulE2EV4` | Stages 1–6 V4, PreDiscovery V4, Traffic/Attribution windows, supporting provenance |
| `TestPrompt96AsynqStages1To6V4` | Redis/Asynq V4 seals + duplicate Core delivery |
| `TestPrompt96AdvertisingTrafficAttributionUseIndependentPeriods` | independent Traffic vs Attribution coverage dates |

Injected factual errors must not become `READY_EMPTY`. Production implemented stages are **1–19** through `POST_PUBLISH` (Prompt №13.3 CLOSED).

## Deep diagnostics (Prompt №12)

Counters/histograms (no seller/SKU/campaign labels): `deep_pack_build_total`, `deep_pack_bytes`, `deep_pack_tokens_estimated`, `deep_ai_attempts_total`, `deep_ai_latency`, `deep_input_tokens`, `deep_cached_input_tokens`, `deep_reasoning_tokens`, `deep_output_tokens`, `deep_thread_resolutions_total`, `deep_findings_total`, `deep_relationships_total`, `deep_recommendations_total`, `deep_action_proposals_total`, `deep_validation_failures_total`, `deep_no_meaningful_threads_total`.

Quality rates (exposed for later evals, not runtime decisions): Discovery substantiation/reframing/not-substantiated/inconclusive via `deep_thread_resolutions_total{resolution}`; evidence-emergent share via `deep_findings_total{origin}`; Action Proposal / ACTIONABLE_NOW via action vs recommendation counters.

Offline quality rubrics: `backend/internal/analysis/deep/fixtures.go`. Live OpenAI is not a unit-CI dependency (`AI_PROVIDER_MODE=real` only).

## Discovery diagnostics (Prompt №10.1)

Counters/histograms (no seller ID labels): `discovery_pack_build_total`, `discovery_pack_bytes`, `discovery_pack_tokens_estimated`, `discovery_fact_handles_total`, `discovery_evidence_menu_types`, `discovery_ai_attempts_total`, `discovery_ai_latency`, `discovery_input_tokens`, `discovery_output_tokens`, `discovery_cached_tokens`, `discovery_validation_failures`, `discovery_threads_total`, `discovery_evidence_requests_total`, `discovery_no_meaningful_threads_total`.

Invariant: visible grounded factual objects == usable visible Fact Handles (except allowlisted structural metadata).

Offline eval fixtures live under `backend/internal/analysis/discovery/testdata/eval/`. Live OpenAI is not a unit-CI dependency (`AI_PROVIDER_MODE=real` only).
