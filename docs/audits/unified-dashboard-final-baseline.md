# Unified Dashboard final baseline (pub 273 / run 408)

**Captured:** 2026-07-17  
**Seller:** `seller_account_id=1`  
**Publication:** `273` (status=`published`, visibility=`visible`, mode=`unified`)  
**Pipeline run:** `408`  
**Metric date:** `2026-07-08` (`date_mode=fixed`, `historical_analysis=true`)  
**Method:** frozen forensic artifacts under `backend/debug/forensic-*`, `backend/debug/_forensic_dash_408.json`, `backend/debug/dash.json`, `backend/debug/act.json`, narrative payload extracts, bundlevalidator/adapter/home source code.

Publication 273 and run 408 are **not mutated** by this audit.

---

## 1. Identity and pointers

| Field | Value |
| ----- | ----- |
| current unified publication | 273 |
| compatibility pointer (`seller_current_ai_snapshots`) | run 408 |
| pack_hash | `9947769e8110a81e04dffc1df3b0e66eb4e16d5d8feb967ba50d22b4e803e767` |
| facts_hash | `0359e46a8491a25525137f94b6edb3fae134c1a46e2c33656027bbfd4400ab23` |
| output_hash | `761df5f9f03c55d88a66938ee9d22355f88d0ba73ad3f374fea2f43d7490774d` |
| bundle_hash | `3eb4e4662a9cdf1ecd9053fa6fbead2cefa0905197dc156b3999894fde629ca1` |
| analysis_outcome | `material_actions_found` |
| analysis_source (API) | `unified` |
| fallback_used | `false` |

---

## 2. Content status (API `/api/v1/dashboard/home`)

| Field | Value |
| ----- | ----- |
| `status` | `ready` |
| `dashboard_content_status` | `ready` |
| `data_quality_notes` | `["Исторический анализ по данным на 08.07.2026"]` |
| KPI count | 4 (`revenue`, `orders`, `ctr`, `drr`) |
| trend_7d count | 7 |
| headline | present |
| executive_summary | present |
| improvement_summary | present |
| health_score | **absent** (`home_has_health_score=false`) |
| what_changed.status | **`source_not_available`** |
| what_changed.message | `AI what_changed не прошёл валидацию.` |
| what_changed.groups | `[]` |
| category_summaries | `[]` (AI emitted 0) |
| stock_coverage | present (5 segments, total_skus=67, narrative ok) |
| ad_efficiency | present (summary + 10 points + narrative) |
| sales_funnel | present (5 stages) |
| funnel_analysis | present (AI short_summary + main_conclusion) |
| risk_distribution | present (3 groups, total=80) |
| top_actions | 3 |
| Actions API count | 4 (`publication_id=273`, `pipeline_run_id=408`) |

Facts artifact counts (run 408): seller_pulse KPIs=4, trend days=7, stock segments=4+, funnel stages=5, ad points=10, sku index=67.

---

## 3. Field missing: `health_score`

| Question | Answer |
| -------- | ------ |
| What is absent? | `account_summary.health_score` (and DB narrative column `health_score`) |
| Where absent? | AI validated output `dashboard.account_summary` keys for run 408: `executive_summary`, `headline`, `improvement_summary`, `metric_date`, `risk_drivers`, `severity`, `status` — **no `health_score`**. Adapter therefore stores `Projection.HealthScore=nil`. Home `AccountSummary` has no `health_score`. |
| Stage | AI output generation / schema optional field; adapter copies only if non-nil |
| Why bundle validator passed | Publication 273 was committed when either (a) `bundlevalidator.checkHealthScore` was not yet wired into the publish path, or (b) the completeness rules were weaker. Current `bundlevalidator.checkHealthScore` **would reject** this bundle with `unified_dashboard_health_score_missing`. |
| Why publication=`published` | Publish path accepted the projection; current pointer switched to 273. |
| Layer | **AI output + publish-time validation gap** (not facts, not frontend) |

Owner (contract): `AI_ANALYSIS` — score + severity required for material outcomes.

---

## 4. Field broken on home: `what_changed`

| Question | Answer |
| -------- | ------ |
| What is broken? | Home widget `ai_diagnostics.what_changed` is empty with `status=source_not_available` |
| Where present upstream? | Stored narrative / AI output **has** what_changed with 4 thematic groups: `revenue`, `ad_efficiency`, `sku_orders_drop`, `slow_turnover` (summary + items present) |
| Where lost? | Home apply path `applyWhatChangedToResponse` → `repairWhatChangedAIOnly` → `ensureWhatChangedRequiredGroups` → `validateWhatChangedPayload` |
| Exact mechanism | Home contract requires keys `worsened`, `new_risks`, `unchanged_risks`, `improved` in that order. Thematic keys are remapped by **positional fallback** (`revenue`→`worsened`, …, `slow_turnover`→`improved`). Content that belongs in a risk/slow-turnover theme is forced into `improved`. Validation then fails `what_changed_improved_items_must_be_empty_when_no_improved_facts` / improved summary phrase rules (and related improved-group checks). On failure the widget is replaced with `source_not_available` + message `AI what_changed не прошёл валидацию.` |
| Why bundle validator passed | `checkWhatChanged` only requires non-nil what_changed with summary/main_conclusion and **any** group with an item. It does **not** require the four home keys. Thematic groups satisfy that weak check. |
| Why publication=`published` | Weak what_changed gate at publish time. |
| Why API content_status stays `ready` | Unified frozen read forces/preserves ready when factual blocks are present; what_changed failure is a **widget-level** status, not a top-level `partial`. |
| Layer | **Contract mismatch**: Unified AI schema allows free-form group keys; home apply requires fixed keys. Loss is at **home apply / validation**, enabled by **weak publish validation**. |

Evidence paths:

- `backend/debug/forensic-baseline-pub273-run408.json` → `what_changed.why_home_missing`
- `backend/debug/_narr_408_what_changed.json` → thematic groups
- `backend/debug/_forensic_dash_408.json` → home widget `source_not_available`
- `backend/internal/dashboardhome/what_changed_types.go` → required keys
- `backend/internal/dashboardhome/narrative_what_changed_repair.go` → positional remap
- `backend/internal/dashboardhome/what_changed_apply.go` → fail → `source_not_available`
- `backend/internal/unifiedanalysis/publication/bundlevalidator/validator.go` → weak `checkWhatChanged`

---

## 5. Yellow banner «Часть данных ещё загружается»

### Exact frontend condition that emits that title

Only `frontend/lib/dashboard-quality-notes.ts` → `buildDashboardQualityBanners`:

1. `readyUnified === false` **and** any non-empty `data_quality_notes` → single warning banner with title «Часть данных ещё загружается»
2. `readyUnified === true` **and** any note classified as `other` (not historical/stale) → warning banner with that title

`readyUnified` is true when `status === "ready"` **or** `dashboard_content_status === "ready"` (`isReadyUnifiedDashboard`).

### Observed API for pub 273

- `status` / `dashboard_content_status` = `ready`
- notes = only historical string matching `/^Исторический анализ по данным на /`

Under **current** frontend code this yields banner key=`historical`, title=«Исторический анализ», tone=`info` (blue) — **not** the yellow loading title.

### What the seller still sees as a yellow/warning surface

1. **WhatChanged widget** (`widget-state.tsx`): `status=source_not_available` uses tone=`warning` (yellow) with message `AI what_changed не прошёл валидацию.`
2. **Account health card**: missing `health_score` shows empty-score copy (`SCORE_EMPTY` in `account-health-card.tsx`): «Индекс пока не рассчитан…»
3. **WhatChanged summary empty path** (`what-changed-view.ts`): `SUMMARY_EMPTY` = «AI-вывод появится после обновления сводки.» when summary missing

These are **not** the string «Часть данных ещё загружается», but they are the remaining seller-facing incomplete/fallback surfaces for this publication.

If an older frontend build still classified historical notes as a generic loading banner, that build would show the exact yellow title for the historical note alone. Current tree no longer does that for ready+historical-only.

---

## 6. Category summaries

| Item | Value |
| ---- | ----- |
| AI `category_summaries` length | 0 |
| Home `category_summaries` | `[]` |
| Bundle validator | `CodeDashboardCategoriesMissing` is **reserved / not gated** (no category-availability signal in `DataCoverage` yet) |
| Layer | AI omitted categories; publish allowed empty array |

---

## 7. Why `partial` is not the top-level API status

Top-level `status` is `ready`, not `partial`. Incomplete analytical widgets (`what_changed`, missing `health_score`, empty categories) do **not** demote content status on the unified frozen read path used for publication 273. That is why a published `ready` dashboard can still show broken analytical blocks.

---

## 8. Root-cause summary (no speculation)

| Defect | Missing / broken field | Absent at | Validator gap | Seller symptom |
| ------ | ---------------------- | --------- | ------------- | -------------- |
| A | `health_score` | AI output → narrative → home | Publish-time health check absent or not enforced for run 408 | Empty health index placeholder |
| B | `what_changed` home widget | Home apply after thematic→positional remap into `improved` | Bundle validator accepts any thematic groups | Yellow widget warning + empty What Changed |
| C | `category_summaries` | AI output | Categories check not enforced | Empty category section |

**Primary root cause of remaining incomplete seller UX on pub 273:** AI analytical contract for `what_changed` / `health_score` is not aligned with the home apply contract, and publication validation for run 408 did not block that incomplete analytical layer. Factual Seller Pulse / funnel / stock / ads blocks are present from backend facts.

---

## 9. Layer attribution

| Layer | Verdict for pub 273 |
| ----- | ------------------- |
| Analysis Pack / snapshot | Not the loss point for the yellow/incomplete analytical UI (facts KPIs/trend/stock/ads/funnel present) |
| Backend facts | Present and projected |
| AI output | Missing `health_score`; thematic `what_changed` keys; empty `category_summaries` |
| Merge / adapter | Copies AI as-is; does not normalize what_changed to home keys; drops nil health_score |
| Bundle validation (at publish of 273) | Did not reject missing health_score / non-home what_changed keys |
| Home API apply | Rejects remapped what_changed; omits health_score |
| Frontend | Renders historical note correctly under current ready path; still shows widget-level warnings/placeholders for broken analytical fields |

---

## 10. Evidence index

- `backend/debug/forensic-baseline-pub273-run408.json`
- `backend/debug/forensic-inspect-pub-273.json`
- `backend/debug/forensic-inspect-bundle-273.json`
- `backend/debug/forensic-facts-408-meta.json`
- `backend/debug/_forensic_dash_408.json`
- `backend/debug/_narr_408_what_changed.json`
- `backend/debug/_run_408_validated_only.json`
- `backend/debug/dash.json`
- `backend/debug/act.json`

## Update (pub 290 / run 409 home-path fixes)

After publication 290 (run 409) shipped with `health_score=80` and correct
what_changed group keys, home still showed `what_changed.status=source_not_available`
and funnel/stock/ads as frontend error shells.

### Additional root causes (proven)

1. **what_changed home validation vs empty PeriodComparison**
   - Unified publications use `buildDashboardNarrativeContextMinimalForAIOnly`
     (datapack rebuild is only for legacy `DashboardNarrativePromptVersion`).
   - That build had `PeriodComparison.Facts = []`.
   - Validation then rejected improved-group items unless period facts existed.
   - Fix: derive PeriodComparison facts from SellerPulse KPI deltas on home apply;
     sanitize invalid highlights.

2. **Factual widget status vocabulary mismatch (`ok` vs `available`)**
   - `dashboardfacts` emitted `status: "ok"` for funnel/stock/ads.
   - Frontend `isWidgetAvailable` / `WidgetState` only treat `available` as ready,
     so UI rendered «Не удалось загрузить блок» despite filled stages/points.
   - Fix: emit/normalize `available`; accept `ok` as alias on FE; normalize on
     `ApplyFrozenDashboardData` for already-published frozen snapshots.

3. **SKU Focus deltas missing on join**
   - Adapter joined stock/coverage only; FE KPI grid requires
     `orders_delta_day_to_day` etc.
   - Fix: map deltas from SKU facts / enrich from frozen `DashboardMetrics.SKUs`
     at read time.

Publication **273** and run **408** were not modified. Current pointer is **290 / 409**.
