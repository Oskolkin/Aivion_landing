# Dashboard completeness contract (Unified Analysis)

**Purpose:** Definition of a publishable `ready` Dashboard + Actions bundle.  
**Rule:** Incomplete bundles are `failed` (not published). `partial` is forbidden for new current Unified publications.

**Owners:** FACT / DERIVED_FACT = backend (`dashboardfacts` from AnalysisDataSnapshot). AI_ANALYSIS = model. PRESENTATION = formatting only.

---

## Publish gate

| Outcome | `content_status` | Publish? |
| ------- | ---------------- | -------- |
| All required blocks present | `ready` | yes → current pointer may switch |
| Any required block missing / placeholder / partial widget | — | **no** → `publication_failed`, pointer unchanged |
| `insufficient_data` / `analysis_failed` | not seller-current material | no material current publish |

Validation error codes: see `bundlevalidator` (`unified_dashboard_*`, `unified_actions_*`, `unified_bundle_*`).

---

## Block / field contract

| Field | Class | Owner | Source | Required (material) | Null OK? | If source missing | Error code | Frontend |
| ----- | ----- | ----- | ------ | ------------------- | -------- | ----------------- | ---------- | -------- |
| analysis_source, publication_id, pipeline_run_id | FACT | backend | analysissource | yes | no | — | cross_source_mismatch | header / meta |
| metric_date, date_mode, historical_analysis | FACT | backend | date resolution | yes | no | — | — | historical banner only |
| pack_id, pack_hash, facts_hash | FACT | backend | pack + facts artifacts | yes | no | fail run | facts_missing / hash | diagnostics |
| content_status | DERIVED_FACT | backend | completeness | must be `ready` | no | fail publish | bundle_partial_not_publishable | quality notes |
| account_summary.headline | AI_ANALYSIS | AI | AI output | yes | no | fail | account_summary_missing | AccountHealth / SellerPulse |
| account_summary.executive_summary | AI_ANALYSIS | AI | AI | yes | no | fail | account_summary_missing | AccountHealth |
| account_summary.improvement_summary | AI_ANALYSIS | AI | AI | yes | no | fail | account_summary_missing | AccountHealth |
| account_summary.risk_drivers | AI_ANALYSIS | AI | AI | yes (≥1 for material_actions_found) | empty array only if healthy + no_material | fail if material_actions and empty risks with actions | account_summary_missing | AccountHealth |
| account_summary.health_score | AI_ANALYSIS | AI | AI 0..100 | **yes** | **no** for material | fail | health_score_missing | AccountHealth score |
| account_summary.severity | AI_ANALYSIS | AI | AI | yes | no | fail | account_summary_missing | AccountHealth |
| account_summary.status | PRESENTATION | backend | widget | available | no | — | — | — |
| what_changed.summary / main_conclusion | AI_ANALYSIS | AI | AI | yes | no | fail | what_changed_missing | WhatChangedCard |
| what_changed.groups | AI_ANALYSIS | AI | AI | **exactly 4 keys:** `worsened`, `new_risks`, `unchanged_risks`, `improved` (order fixed) | no | fail | what_changed_missing | WhatChangedCard |
| what_changed group items | AI_ANALYSIS | AI | AI + facts deltas | at least one group with items when PoP possible | empty group uses empty_text | if prior period missing → do not publish material fixed acceptance | what_changed_missing | WhatChangedCard |
| seller_pulse.kpis revenue/orders/ctr/drr | PRESENTATION of FACT | backend | facts | yes if AccountMetricsDays>0 | no invented | fail | kpis_missing | SellerPulse KPI cards |
| seller_pulse.trend_7d | FACT | backend | facts | yes if coverage | no zero-fill missing days | fail | trend_missing | SellerPulse trend |
| seller_pulse.narrative | AI_ANALYSIS | AI | AI | yes | no | fail | seller_pulse_missing | SellerPulse narrative |
| category_summaries | AI_ANALYSIS | AI | AI (+ category facts when available) | required when `DataCoverage.CategoryCount>0` / category facts non-empty | empty only if no categories in pack | fail when categories exist and array empty or incomplete (missing category_score/headline) | unified_dashboard_category_analysis_missing / categories_missing | Category cards |
| category facts | FACT | backend | dashboardfacts from Products+Categories+SKU metrics | required when catalog has categories | fail facts build if catalog categorized but facts empty | fail pre-OpenAI | unified_dashboard_category_facts_missing | — |
| sku_focus lists + entity metrics | hybrid | AI pick + backend facts join | facts index + AI | ≥1 entry if SKU index non-empty | no fake SKUs | fail | sku_focus_missing | SKUFocusSection |
| sku_focus.sparkline_7d | FACT | backend | SKU daily metrics (missing days HasData=false) | required when SKUMetricsDays≥2 for each published SKU focus card | no zero-fill | fail | unified_dashboard_sku_trend_missing | SKU sparkline |
| stock_coverage.segments / totals | DERIVED_FACT | backend | facts | yes if StockSnapshotRows>0 | explicit not_enough_data OK | fail | stock_missing | Stock widget |
| stock_coverage.narrative | AI_ANALYSIS | AI | AI | yes when stock factual present | no | fail | stock_missing | Stock narrative |
| ad_efficiency.summary / points | DERIVED_FACT | backend | facts | yes if AdMetricsDays>0 | explicit no-data OK | fail | ads_missing | Ads widget |
| ad_efficiency.narrative | AI_ANALYSIS | AI | AI | yes when ads factual present | no | fail | ads_missing | Ads narrative |
| ad_efficiency.narrative.checklist | AI_ANALYSIS | AI | AI ≥2 concrete items when ads present | **yes** when AdMetricsDays>0 | no | fail | unified_dashboard_advertising_checklist_missing | Ads checklist |
| sales_funnel.stages | DERIVED_FACT | backend | facts | yes if FunnelAvailable | no | fail | funnel_missing | Funnel |
| funnel_analysis.ai_analysis | AI_ANALYSIS | AI | AI | yes when stages present | no | fail | funnel_analysis_missing | Funnel analysis |
| risk_distribution.groups | hybrid | backend counts preferred; AI labels | facts/alerts/AI | yes for material_actions_found | empty OK for no_material_actions | fail | risk_distribution_missing | RiskDistribution |
| top_actions | AI_ANALYSIS | AI | AI actions | ≥1 for material_actions_found | empty for no_material | fail | top_actions_missing | Top actions |
| actions[*] full detail | AI_ANALYSIS | AI | AI | ≥1 for material_actions_found; steps + completion_criteria required | — | fail | actions_missing / action_detail_incomplete | Actions list/detail |

---

## what_changed key contract (home-compatible)

| Key | Label (RU) | Tone | Meaning |
| --- | ---------- | ---- | ------- |
| `worsened` | Ухудшилось | negative | Metrics that deteriorated vs prior period |
| `new_risks` | Новые риски | warning | Newly appeared risks |
| `unchanged_risks` | Активные проблемы | neutral | Ongoing unresolved problems |
| `improved` | Улучшилось | positive | Genuine improvements only |

Thematic keys (`revenue`, `ad_efficiency`, `sku_orders_drop`, …) are **invalid** for publish and for home apply.

---

## health_score ownership

`health_score` is **AI_ANALYSIS** (0..100) with required `severity` and risk_drivers context.  
Backend does not invent a hidden formula. AI must supply score + explanation via executive/risk narrative. Bundle rejects nil/out-of-range.

---

## Merge policy

1. FACT / DERIVED_FACT always win over AI numbers.  
2. AI text only on AI_ANALYSIS fields.  
3. Unknown entity IDs → validation failure.  
4. Strong numeric mismatch → warning (or failure per policy); published value remains backend.

---

## Frontend ready Unified rules

For `status=ready` / `dashboard_content_status=ready` and `analysis_source=unified`:

- Show historical note as info only (`Исторический анализ…`).
- Never show «Часть данных ещё загружается», generation placeholders, or metric-sync placeholders for complete blocks.
- If API incorrectly returns ready with broken required analytical widgets → show explicit technical error (do not mask with loading banner).

Forbidden seller strings on ready Unified:

- Часть данных ещё загружается  
- Метрика появится после синхронизации  
- AI-вывод появится после генерации  
- AI dashboard not generated  
- Генерация неполная / выполняется  
- legacy v9 generation messages  
