# Dashboard full audit baseline — 2026-07-21

**Baseline commit:** `e0dfc3227e9d92b2a28a8590fec7090d651907a4` (`Revert "Переработка генерации данных дашборда 4"`)  
**Branch:** `main`  
**Schema / prompt:** `aivion_unified_analysis_v8` / `unified_analyst_v1_22`

This artifact is **not mutated** after capture. Implementation follows in the same change set.

---

## 1. Field lineage (summary)

| Блок | Поле | Текущий источник | Текущий период | Владелец | Проблема | Целевой источник | Требуемое изменение |
| ---- | ---- | ---------------- | -------------- | -------- | -------- | ---------------- | ------------------- |
| Seller Pulse | KPI revenue | `dashboardfacts` ← AccountMetrics | current_day vs previous_day | FACT | OK | same | — |
| Seller Pulse | KPI orders | `dashboardfacts` ← AccountMetrics | current_day vs previous_day | FACT | OK | same | — |
| Seller Pulse | KPI ctr | FunnelRows CTR | current_day | DERIVED_FACT | **CTR вместо AOV** | AOV = revenue/orders | заменить KPI |
| Seller Pulse | KPI drr | AccountDRRPct(spend, store revenue) | current_day | DERIVED_FACT | ключ/лейбл «ДРР» путает с attributed DRR | `ad_spend_share_total_revenue` | rename + RU label |
| Seller Pulse | narrative | AI Unified output | current_7d vs previous_7d (claims) | AI_ANALYSIS | OK при claims | same | усилить validator |
| Health | health_score | **AI** account_summary | publication | AI_ANALYSIS | должен считать backend | DERIVED_FACT + breakdown | compute at publish |
| Health | breakdown | legacy only / env-gated | — | hybrid | на Unified не публикуется | backend breakdown | expose always |
| What Changed | groups/items | AI | 7d + publication_comparison | AI_ANALYSIS | GET strengthen/derive | AI frozen | убрать semantic repair на GET |
| Top 3 | top_action_ids | AI → Actions | same run | AI_ANALYSIS | GET может override из actions | same Actions at publish | freeze Top 3 in bundle |
| Funnel stages | values/conversions | dashboardfacts | current_day | DERIVED_FACT | OK | same | — |
| Funnel AI | narrative | AI | mixed day/7d risk | AI_ANALYSIS | period must be explicit | AI + period meta | prompt/validator |
| Stock segments | buckets | dashboardfacts exclusive | metric_date | DERIVED_FACT | OK | same | — |
| Ads summary | attributed DRR | analytics ad efficiency | day + 7d | DERIVED_FACT | OK after dual-metrics work | same | — |
| Ads narrative | % text | AI + **adapter rewrite** | 7d | AI + PRESENTATION mutate | post-AI text rewrite | claims only | prefer reject vs rewrite |
| SKU Focus | key_skus | **AI pick** | — | AI | нет 80% когорты store revenue | backend cohort + AI text | join backend key set |
| SKU Focus | sparklines | sku_daily facts | trend_7d | DERIVED_FACT | частично закрыто | same | verify |
| Categories | cards | AI shells | — | AI | empty shells drop | optional / hide | hide empty |
| Recommended Actions | list | deprecated | — | — | API field still present | omit from FE | keep deprecated API only |
| Period context | captions | GET finalize from metric_date | day/7d | PRESENTATION | OK | same | allow frozen override |
| Identity | pack/facts/run/pub | snapshot + source | — | FACT | facts_hash naming soft | explicit facts_hash | stamp on frozen |

---

## 2. Gap lists (required)

### AI генерирует, хотя должен считать backend
1. `health_score` / `health_score_delta`
2. Seller Pulse KPI arrays in schema (ignored, but still solicited)
3. `key_skus` membership / `revenue_share_pct` (AI may invent share)
4. Risk distribution counts (когда нет alert-backed фактов)

### Backend пересчитывает после AI
1. `healthstatus.FromScore` severity remap (publish + GET)
2. Ad narrative % grounding (`groundStoreRevenueShareMentions`)
3. What Changed group summary synthesis in adapter
4. SKU Focus fact join / drop unresolved

### Изменяются после публикации (GET BuildHome)
1. `finalizeSellerFacingDashboardResponse` (health bands, WC strengthen, quick actions rebuild, surplus→slow_turnover)
2. `applyWhatChangedToResponse` repair/derive/strengthen
3. `overrideTopActionsFromFinalActions`
4. `enrichFrozenSKUFocusFromMetrics`
5. Stock/pulse factual fallbacks on legacy/AI-only paths

### Неоднозначный период
1. Funnel stages = day; Funnel AI may discuss 7d without label
2. Pulse AI claims vs day KPI cards (mitigated by period_context)
3. Prompt appendix mislabels `account_drr_pct` as attributed

### Дублируются
1. Pulse important_facts ↔ What Changed / risk drivers (signal ownership partially enforced)
2. Recommended Actions API field vs Top 3 (deprecated but present)
3. Category summaries vs SKU Focus / diagnostics

### В contract, не отображаются / теряются
1. What Changed `signal_id` / `fact_ids` dropped in narrative projection
2. `health_score_breakdown` not on Unified contract
3. Schema enum missing `seller_pulse_key_point`

### Отображаются без надёжного источника
1. Empty category shells historically published then dropped
2. Legacy deterministic What Changed / Top Actions fallbacks

### Legacy влияет на Unified
1. `enforceAIOnlyGenerationRequired` category readiness gate (skipped for unified stamp, but shared finalize remains)
2. Surplus→SlowTurnover remap on every GET
3. analysissource legacy fallback when unified not visible

### Fallback, скрывающие ошибку
1. `runBlock` panic recover
2. Legacy WC deterministic substitute
3. Stock factual narrative substitute
4. Legacy Top Actions fallback
5. AI-only status/summary synthesis

### Данные другого run
1. Narrative load by metric date if DashboardRunID miss (legacy)
2. Top Actions override from actions pipeline run vs narrative run (mitigated when same publication)

---

## 3. Post-publication mutation table (priority)

| Function | What changes | Move before publish? | Delete? |
| -------- | ------------ | -------------------- | ------- |
| `strengthenWhatChangedMainConclusion` / derive | Main conclusion text | Yes | Soft — skip on unified GET |
| `overrideTopActionsFromFinalActions` | Top 3 list | Freeze at publish | Soft for unified |
| `localizeSellerPulseKPILabels` | Labels | Facts builder | Keep as presentation |
| `applyHealthStatusFromScore` | Severity labels | Publish | Keep as presentation |
| `omitDeprecatedSKUFocusSurplus` | Tab remap | Freeze | Soft |
| Ad % grounding in adapter | Narrative numbers | Prefer validator reject | Soft |

---

## 4. Implementation order for this change set

1. Seller Pulse: AOV + ad spend share key/label (facts + FE + validation)
2. Backend health score + breakdown at `ProjectDashboard`
3. Key SKU 80% cohort join on unified path
4. Skip semantic WC strengthen on unified GET
5. Schema/prompt: `seller_pulse_key_point` enum; fix `account_drr_pct` grounding text
6. Update ownership doc + tests
