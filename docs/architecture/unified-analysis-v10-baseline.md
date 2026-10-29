# Unified Analysis v10 baseline (repository audit)

**Baseline SHA:** `1f6a0670970abd01eaff6d6457a1e2e0d21293e8` (branch `main` / `origin/main` at audit start)  
**Feature branch:** `feat/unified-analysis-v11-contract`  
**Audit date:** 2026-08-06  
**Source of truth:** runtime Go code under `backend/internal/unifiedanalysis/` (docs consulted only where they match code).

---

## 1. Production versions (unchanged by Part 1)

| Kind | Value |
| --- | --- |
| Schema (current) | `aivion_unified_analysis_v10` |
| Schema hash | `sha256:8c2474226f0de69ec18709df7da9853fa5fb11e00522caf829e270bc3ecc673b` |
| Prompt (current) | `unified_analyst_v1_31` |
| Pack schema | `aivion_analysis_pack_v1` |
| Pipeline mode | `unified_analysis` / `unified_analysis_v1` |
| Constants | `contract.UnifiedAnalysisOutputSchemaVersionCurrent = V10`; `prompt.CurrentAnalystVersion = VersionAnalystV1_31`; `unifiedanalysis.PromptVersionAnalyst = unified_analyst_v1_31` |

There is no separate machine-readable `analysis_policy_v2` in production. Pack ships `analysis_policy.json` from `contract.DefaultAnalysisPolicyV1()` / `pack.BuildAnalysisPolicy()` (limits + allowed enums only). Comparative thresholds live in `thresholdpolicy.Registry` (Go), not in the Pack policy JSON.

---

## 2. Action limits and types

| Limit | Value | Location |
| --- | --- | --- |
| Max Actions | **15** | `contract.MaxActionsV1` |
| Max Top Actions | **3** | `contract.MaxTopActionsV1` |
| Min Actions (material) | **1** | `MinActionsMaterialV1` (floor after quota removal) |
| Min required (material) | **1** | `MinRequiredActionsMaterialV1` |

**Action types (13)** — `contract.AllowedActionTypesV1`:

`stock_replenishment`, `stock_redistribution`, `slow_turnover_review`, `no_sales_with_stock`, `sku_orders_drop`, `key_sku_protection`, `ad_high_drr_review`, `ad_spend_no_orders`, `ad_budget_reallocation`, `funnel_weak_transition`, `card_conversion_review`, `category_underperformance`, `sales_drop_review`

**Categories:** `stock`, `slow_turnover`, `sku`, `sales`, `advertising`, `funnel`, `category`  
**Priorities:** `critical`, `important`, `watch`  
**Execution tiers:** `required`, `recommended`  
**Scope types (v10 AI):** `account`, `sku`, `campaign`, `funnel`, `category`, **`mixed`**  
**Outcomes:** `material_actions_found`, `no_material_actions`, `insufficient_data`, `analysis_failed`

---

## 3. Thresholds (production)

| policy_id | metric_code | scope | op | value | File |
| --- | --- | --- | --- | --- | --- |
| `high_drr_campaign_threshold` | `campaign_drr_pct` | campaign | gt | **80** | `thresholdpolicy/policy.go` |
| `low_stock_coverage_days_threshold` | `stock_coverage_days_7d` | sku | lt | **7** | same |

Note: dashboard “high DRR” campaign lists use `analytics.AdEfficiencyHighDRRThresholdPct = 100` — different from AI cite threshold 80.

---

## 4. Dashboard output contract (v10 AI shape)

Top-level: `schema_version`, `analysis_meta`, `analysis_outcome`, `dashboard`, `actions`, `limitations` (string[]).

Dashboard blocks:

| Block | Role |
| --- | --- |
| `account_summary` | Headline, executive/improvement text, health_score, severity, risk_drivers, levers |
| `what_changed` | Groups + free-text change cards |
| `category_summaries[]` | Per-category score/severity/headline/risk/action |
| `sales_funnel` | Stages (often overwritten by facts) |
| `funnel_analysis` | AI funnel narrative (`ai_analysis`) |
| `stock_coverage` | Segments + narrative |
| `ad_efficiency` | Factual summary + AI narrative (+ metric_claims, threshold_claims) |
| `risk_distribution` | Groups / counts |
| `seller_pulse` | KPIs/trend (facts) + narrative (AI) |
| `sku_focus` | Five lists of SKU cards |
| `top_action_ids` | Up to 3 |
| `recommended_action_ids` | Remainder |

Action shape includes AI-owned `category`, free-text `summary`/`problem`/`why_now`, `steps`, `completion_conditions` with **AI-provided** `baseline_value` / `operator` / `target_value`, `evidence_fact_ids`, `source_signal_ids`.

---

## 5. Ownership model (code-backed)

Canonical merge: `publication/adapter` + `docs/current/dashboard-field-ownership.md` (aligned).

| Owner | Fields |
| --- | --- |
| Backend facts | Seller Pulse KPIs/trend; stock segments/totals; ad factual summary; funnel stages; SKU metrics/names; health_score at publish; date/freshness; pack hashes |
| AI | Narratives, headlines, action prose, entity selection for sku_focus, risk_driver copy, what_changed copy, category analytical text, top/recommended IDs (then repaired) |
| Hybrid | Risk distribution; category scores after enrichment |

Rule: FACT/DERIVED_FACT never overwritten by AI numbers; missing facts → null / not_enough_data, not silent AI fallback.

---

## 6. Stage-4 post-processing (actual behavior)

Pipeline in `validate_output.go` after parse:

1. **SanitizeGrounding** — overwrite pack meta from run; drop unknown/dup SKU entities; promote empty sku/campaign scope → account; rewrite completion targets already met (±10% of \|baseline\|, min 1); clear duplicate what_changed signal_ids.
2. **SyncActionEntityCounts** — rewrite free-text entity counts; expand thin expected_effect / plans / why_now.
3. **RepairUnsupportedContentSteps** — rewrite photo/content causal steps (content evidence always false today).
4. **ReconcileStockActionCohorts** — **delete all** AI stock cohort Actions (`stock_replenishment` / `stock_redistribution` / `key_sku_protection`).
5. **EnsureMaterialDomainActions** — **append** Pack-grounded synthetic Actions for OOS/critical/short/sales/ads/no-sales/surplus.
6. **EnsureCategorySummaries** — if empty, **add** placeholder cards (`score=50`, `severity=watch`, generic RU text).
7. **ScrubInternalFieldNames** — replace internal metric tokens in action copy.
8. **SyncActionEntityCounts** (again).
9. **FinalizeTopActionIDs** — rebuild top list (required only), demote recommended, materiality reorder.
10. **FinalizeDashboardRiskDrivers** — normalize/merge risk driver IDs to health_policy registry.
11. **FinalizeDashboardSectionFactReferences** — strip non-account facts from Seller Pulse.
12. **Validate** — hard contract (also re-syncs counts).

Publication adapters then replace factual dashboard fields from Dashboard Facts and recompute health score via `healthpolicy`.

---

## 7. Fields AI returns that backend then mutates

| Treatment | Examples |
| --- | --- |
| Recalculates | `health_score` / severity at publish; priority ordering of Top Actions |
| Replaces | `analysis_meta.*`; completion `target_value` when already met; thin `expected_effect` / plans; content steps; risk_driver IDs; free-text entity counts |
| Deletes | Unknown/dup entity_ids; all AI stock cohort Actions; invalid Seller Pulse entity facts; non-required from top_action_ids |
| Supplements | Domain Actions via EnsureMaterialDomainActions; category_summaries placeholders; Top Action backfill from required list |
| Reorders | `top_action_ids` by materiality; publish-time required-before-recommended |

---

## 8. Feature flags / rollout

Orchestration (`ConfigFromEnv`): `UNIFIED_ANALYSIS_PROVIDER_ENABLED` (alias `UNIFIED_ANALYSIS_ENABLED`), model/prompt/token/poll/cleanup/debug flags.  
Publication (`GlobalFlagsFromEnv`): `UNIFIED_ANALYSIS_ROLLOUT_ENABLED`, `UNIFIED_ANALYSIS_AUTO_RUN_ENABLED`, `UNIFIED_ANALYSIS_AUTO_PUBLISH_ENABLED`, `UNIFIED_ANALYSIS_LEGACY_FALLBACK_ENABLED` (default true).  
Per-seller: `seller_analysis_settings.analysis_mode` ∈ `legacy|shadow|unified`.

---

## 9. Provider artifacts (kinds)

`unified_provider_request_summary`, `unified_provider_response_raw`, `unified_provider_response_summary`, `unified_provider_usage`, `unified_provider_terminal_attempt`, `unified_provider_cleanup`, `unified_analysis_pack_reference`, `unified_analysis_date_resolution`, `unified_dashboard_facts_v1`, `unified_pack_generation_validation`, `unified_source_reconciliation_v1`, `unified_analysis_output_raw`, `unified_analysis_output_validation`, `unified_analysis_output_validated`, `unified_analysis_output_summary`.

---

## 10. Source resolver / fallbacks

- Metric date: `analysisdata.ResolveAnalysisMetricDate` → artifact `unified_analysis_date_resolution`.
- Pre-provider: `sourcerecon.ReconcileSnapshot` gate.
- Validate response: artifact raw first, else live Retrieve.
- Publication policy: hardcoded accepted prompt/schema hashes if live render fails.
- Dashboard facts win over AI numbers; **no AI numeric fallback**.
- Content evidence permanently unavailable until Pack enrichment.

---

## 11. Problems of the current v10 scheme

1. AI still invents / writes raw business numbers in many narratives; grounding is regex/claim-heavy repair.
2. AI returns backend-owned facts (health_score, funnel stages, stock segments, KPIs, ad summary, category scores, Top Actions, risk distribution).
3. Heavy silent repair: stock Actions deleted and regenerated; category score=50; scope promotion; target+10%; SyncActionEntityCounts invents plan depth.
4. Dashboard ↔ Actions linkage is weak (IDs only); no shared Finding model → contradictions and duplicates.
5. Prompt pushes “prefer 8–12 Actions” (v1_31) → pressure to invent volume.
6. `mixed` scope and AI-owned `category` blur responsibility.
7. Completion targets invented by AI then repaired arbitrarily.
8. Limitations are free strings / loose codes rather than structured affected_blocks.
9. Large output + large Stage-4 surface → cost, latency, and hidden semantic mutations.
10. Thresholds split across Go registry vs Pack policy JSON vs prompt text.

---

## 12. Fields removed from AI contract in v11

(Non-exhaustive; see mapping doc.)

- `dashboard.account_summary.health_score` / `health_score_delta` / `severity` / `risk_drivers` / `metric_date`
- `dashboard.risk_distribution` (entire block)
- `dashboard.top_action_ids` / `recommended_action_ids`
- AI factual funnel stages, stock segments/totals, ad summary metrics, Seller Pulse KPIs/trend
- Category `category_score` / `severity` / `category_name` / generic key_risk / recommended_action
- Action `category` (backend-derived)
- Action free-form completion `target_value` / AI baseline inventing
- Scope type `mixed`
- Ambiguous block statuses `ok` / `partial` / `error` → `available` / `limited` / `unavailable`
- Raw numeric business literals in user-facing templates

---

## 13. Functions changed or removed from the v11 path (Part 2; v10 kept until pilot)

| Function | v11 fate |
| --- | --- |
| `SanitizeGrounding` | Narrow to trim/order/exact-dup only; **no** silent scope promotion / target+10% |
| `SyncActionEntityCounts` | Out of v11 path (no free-text count repair) |
| `RepairUnsupportedContentSteps` | Keep as secondary safety or drop once Pack/content policy complete |
| `ReconcileStockActionCohorts` | Replace with coverage validation (no delete-all + regenerate) |
| `EnsureMaterialDomainActions` | → `ValidateMaterialFindingCoverage` (requirements, not synthetic Actions) |
| `EnsureCategorySummaries` | Removed from v11 path (no score=50) |
| `ScrubInternalFieldNames` | Keep as hard reject (not silent rewrite) where possible |
| `FinalizeTopActionIDs` | → backend Priority Policy assembler |
| `FinalizeDashboardRiskDrivers` | → backend from findings |
| `FinalizeDashboardSectionFactReferences` | → Section Relevance Validator (fail closed) |

---

## 14. Part 1 deliverables (additive only)

- Schema `aivion_unified_analysis_v11` registered, **not** current
- Prompt `unified_analyst_v1_32` registered, **not** current
- Policy `analysis_policy_v2` machine-readable, not wired into production runs
- Docs, fixtures, contract/semantic tests

Production continues on v10 / v1_31 until Part 2 gates pass.
