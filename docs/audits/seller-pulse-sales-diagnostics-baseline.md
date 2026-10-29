# Seller Pulse + Sales Diagnostics baseline (pub 11 / run 16)

**Captured:** 2026-07-18  
**Seller:** `seller_account_id=1`  
**Publication:** `11` (status=`published`, visibility=`visible`, mode=`unified`)  
**Pipeline run:** `16`  
**Metric date:** `2026-07-08` (`date_mode=fixed`)  
**Prompt version:** `unified_analyst_v1_12`  
**Schema version:** `aivion_unified_analysis_v1`  
**Method:** `inspect-unified-publication --publication-id=11`, `inspect-unified-analysis-output --run-id=16`, `debug/_accept2_run16_validated.json`, `debug/_accept2_home.json`, source of adapter / home finalize / frontend.

Publication 11 and run 16 are **not mutated** by this audit.

---

## 1. Identity

| Field | Value |
| ----- | ----- |
| publication_id | 11 |
| run_id | 16 |
| pack_hash | `41202f3bd90fe927c1ef598c2194258d852c2978f66c80e1c8874cfd05587c3b` |
| facts_hash | `ba862ddd363801c622362f8d97ef08fb38c768f3358a6013e9f9be6f69395916` |
| output_hash | `e07c6e4f33e7eee65161e05109ac30001a255efd9dab7475e6607f5d7db13341` |
| bundle_hash | `175e9bbc528c26ec4bad2bc2c613470f7435fe2587ef1b4e2b1cb002e473662e` |
| analysis_outcome | `material_actions_found` |
| actions_count | 3 |
| dashboard_content_status | `ready` |

---

## 2. Field source map

| UI field | Source layer | Notes |
| -------- | ------------ | ----- |
| Seller Pulse KPI values / deltas / sparklines | **backend fact** | `dashboardfacts` → adapter `projectSellerPulseFromFacts` wins over AI |
| Seller Pulse headline / summary | **AI output** | `dashboard.seller_pulse.narrative` |
| «Что важно знать» (`important_notes`) | **adapter / home finalize** | AI schema has **no** `important_notes`; home copies `risk_drivers` when empty (`buildSellerPulseImportantNotesFromRiskDrivers`) |
| «Быстрые действия» | **adapter / home finalize** | Built from Top Actions; **href** remapped via `sellerPulseKindForTopAction` to legacy routes |
| Health score number | **AI output** | `account_summary.health_score=75` |
| Health status badge text | **frontend-generated** | `getHealthSeverityLabel(severity)`; AI severity=`warning` → «Статус не определён» |
| «Основные причины снижения» | **AI output** | `risk_drivers` |
| «Что улучшит индекс» | **AI output** (weak) | `improvement_summary` = «Материальных улучшений…» — not levers; empty/filtered → frontend stub |
| What changed groups / main_conclusion | **AI output** | groups present; main_conclusion ~93 chars |
| Period labels on KPIs | **mixed** | facts delta often day-over-day; AI prose uses «за неделю»; charts are 7d |
| Number formatting in cards | **frontend / facts display** | AI prose uses mixed `₽` / `руб.` / `--6.0%` |
| DRR dashed «Норма 30%» line | **frontend-generated** | `PulseKpiChart` `showDrrNorm` |

---

## 3. Persisted AI snapshot (run 16)

### Seller pulse narrative

- **headline:** «Требуется срочная работа по остаткам и корректировка рекламы»
- **summary (99 chars):** «Риски по остаткам и рекламе — основные точки роста. Выручка за неделю: 975 390₽, заказы: 672 (-6%).»
- Missing: causal picture, balancing positive factor, explicit priority, 3–5 analytical sentences.

### Account summary / index

- **health_score:** 75  
- **severity (AI):** `warning` ← not in FE enum `{critical,high,medium,attention,low}`  
- **executive_summary:** ~338 chars, weekly dynamics + stock/ads risks (contains English `DRR`)  
- **improvement_summary:** «Материальных улучшений по ключевым метрикам за неделю не выявлено.» (not improvement levers)  
- **risk_drivers:** 3 — orders/revenue drop; stockout; high DRR campaigns

### What changed

- **main_conclusion:** ~93 chars — imperative priority only, not synthesis  
- **worsened:** orders −6%, revenue −7.8% (week)  
- **new_risks:** SKU order drop; stock deficit (no previous-publication proof; only one published pub)  
- **unchanged_risks:** high DRR campaign (labeled «active» without prior pub)  
- **improved:** empty → no support points despite normal cabinet-level DRR available in facts

### Actions (top)

1. `action_001` stock_replenishment (critical)  
2. `action_002` sku_orders_drop  
3. ads high-DRR review (title/problem contain `DRR`)

---

## 4. Root causes of observed defects

| # | Defect | Root cause |
| - | ------ | ---------- |
| 1 | Short Seller Pulse AI conclusion | Prompt/schema only require short `headline`+`summary`; no depth/structure contract; no min semantic coverage validation |
| 2 | Duplicate analytical signals across blocks | Home **copies** `risk_drivers` → `important_notes`; what_changed / drivers / pulse share same themes without `signal_id` / `primary_section`; no cross-section dedupe validator |
| 3 | Index present but status «не определён» | AI emits non-canonical `severity=warning`; status label is **frontend-mapped** from AI severity instead of **backend-derived** from `health_score` |
| 4 | Stub «Что улучшит индекс» | No structured `improvement_levers`; `improvement_summary` is a negation, not actionable levers tied to drivers+Actions; FE shows `IMPROVEMENT_EMPTY` when empty/filtered |
| 5 | English terms (`AI`, `DRR`, `CTR`, «селлер») | Prompt allows CTR/DRR in analyst English; FE labels `CTR`, title «AI-диагностика» / «Пульс селлера»; AI texts emit `DRR`; no RU presentation validator |
| 6 | Quick Actions → obsolete sections | `sellerPulseAllowedKinds` / `sellerPulseKindForTopAction` hardcode `/app/advertising`, `/app/products`, `/app/stocks-replenishment`, `/app/critical-skus`; FE fallback `buildPulseQuickActions` same; Top Actions CTA also falls back to legacy routes |
| 7 | Dashed DRR threshold line | Frontend `pulse-kpi-chart.tsx` draws `Норма 30%` with `strokeDasharray` for `metricKey==="drr"` |
| 8 | Mixed day / week periods | Facts KPI deltas often vs previous **day**; AI narrative talks **7d vs prior 7d**; sparklines are 7d; no structured `period_context` on API; FE does not label comparison scope consistently |
| 9 | Cabinet DRR vs campaign DRR unexplained | Prompt does not require distinguishing account-level DRR vs local campaign exceptions; AI treats high campaign DRR as global advertising failure without reconciliation |

---

## 5. Contract gap summary (pre-fix)

| Layer | Gap |
| ----- | --- |
| Analysis Pack | Periods/previous publication summary not explicitly required in analyst instructions for status `new`/`active` |
| AI prompt | No block role separation; no depth rules; English metric terms; no DRR reconciliation; no signal ownership |
| Schema | No `important_facts`, `improvement_levers`, signal metadata, period refs |
| Output validator | No shallow-conclusion / dedupe / RU-term / unjustified new\|active checks |
| Publication adapter | Narrative projection drops notes; does not attach action hrefs / health status from score |
| Bundle validator | Does not enforce levers / support points / signal uniqueness |
| Dashboard API | Severity passed through; quick action hrefs legacy; no period_context |
| Frontend | Status from AI severity; stubs; CTR label; DRR norm line; legacy QA fallbacks; titles not fully RU |

---

## 6. Acceptance target after fix

New Unified Analysis run for seller 1 / `2026-07-08` with bumped prompt+schema versions; published as current; old pub 11 retained. Both blocks filled without stubs; Quick Actions → `/app/actions?action=…`; health status from score; no DRR dashed norm line; RU presentation except `SKU`.
