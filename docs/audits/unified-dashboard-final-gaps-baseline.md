# Unified Dashboard final gaps baseline (pub 290 / run 409)

**Captured:** 2026-07-17  
**Seller:** `seller_account_id=1`  
**Publication:** `290` (`pipeline_run_id=409`, `metric_date=2026-07-08`)  
**Method:** code + artifacts (`backend/debug/_run_409_*`, `_accept_dash_290*`, `_pw_smoke.txt`) + [forensic explore](d6e8a538-5766-4ca8-9509-a39c4b79a48c).

Publication 290 / run 409 are **not mutated**.

---

## Gap 1 — Missing `category_summaries` / category cards

| Layer | State |
| ----- | ----- |
| Pack ZIP | Category ids/names on SKUs; `category_count` in manifest |
| Facts | No category facts / no category coverage signal |
| AI (409) | 5 shells: only `category_id`/`category_name`/`score_max`; scores/texts null |
| Home apply | `normalizeCategorySummaryCardsAIOnly` drops cards without `category_score` → `[]` |
| Bundle validator | `CodeDashboardCategoriesMissing` reserved, never enforced |
| UI | Empty array → section not rendered |

**First loss:** incomplete AI category cards; home apply empties array.  
**Owner:** AI contract + home gate + validator gap. Pack has category identity.

---

## Gap 2 — Missing advertising AI checklist

| Layer | State |
| ----- | ----- |
| Unified schema | `UnifiedAdEfficiencyNarrativeV1` has status/summary/limitations only — **no checklist** |
| Adapter | Never maps checklist |
| Bundle validator | Checks factual ads summary only |
| UI | Empty checklist → «AI-чеклист по рекламе пока недоступен» |

**First loss:** Unified AI schema.  
**Owner:** AI contract/adapter. Pack has campaign facts.

---

## Gap 3 — Missing SKU Focus 7-day sparkline

| Layer | State |
| ----- | ----- |
| Pack ZIP | `sku_daily.csv` present |
| Facts / adapter | `SKUFactRecord` has no sparkline; join never sets `Sparkline7D` |
| Bundle validator | Requires ≥1 SKU card only |
| UI | «Нет данных для тренда» |

**First loss:** backend facts/merge.  
**Owner:** FACT/DERIVED_FACT. Pack has daily series.

---

## Gap 4 — Why publish succeeded

`ValidatePublicationBundle` gates KPIs/trend/funnel/stock/ads facts/SKU non-empty/health/what_changed — **not** categories completeness, ad checklist, or SKU sparklines.

---

## Gap 5 — Playwright «no current unified publication»

1. Isolated seed runs legacy AI pipeline only — no Unified facts artifact.  
2. `seedPublicationReadyFixture` seeds validated AI output without `unified_dashboard_facts_v1`.  
3. `PublishRun` fails bundle validation → pointer unchanged (no Go error).  
4. `findOrPublishFixture` ignores `OutcomeBundleValidationFailed`.  
5. `switchToPublication` always `RollbackPublication`, which requires an existing current pointer.

---

## Gap 6 — `date_mode=current` isolated

No pack+facts+visible publish for latest-complete on isolated seller; fixture dates are hardcoded; Rollback cannot bootstrap first current pointer.

---

## Evidence index

- `backend/debug/_run_409_output.json`
- `backend/debug/_accept_dash_290.json`
- `backend/debug/_pw_smoke.txt`
- `bundlevalidator/validator.go`, `adapter/dashboard.go`, `dashboardfacts/types.go`
- `unified_browser_fixtures_devtool.go`, `publication/rollback.go`
- FE: `ad-efficiency-display.ts`, `sku-focus-section.tsx`, `category-status-cards-section.tsx`
