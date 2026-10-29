# Dual advertising spend-share metrics — final report

Date: 2026-07-20  
Seller: 1 · Metric date: 2026-07-08  
**Current publication: 41 (run 63)** · Previous baseline pub: 34 (run 55)

## 1. Root cause

Two blocks historically mixed (or forced equal) two different business ratios that share the same numerator (all-campaign ad spend) but different denominators:

| Denominator | Source | 7d value |
|-------------|--------|----------|
| Total store / cabinet revenue | `daily_account_metrics.revenue` | ≈ 975 390 ₽ |
| Attributed ad revenue | `ad_metrics_daily.revenue` | 178 500 ₽ |

Same spend 163 800 ₽ → **16,8%** vs **91,8%**. AI sometimes mixed spend + attributed revenue while quoting the store share.

## 2–4. Semantics, formulas, campaign scope

| Block | Metric | Formula | Scope |
|-------|--------|---------|-------|
| Пульс продавца | Доля рекламных расходов в **общей** выручке | `total_ad_spend / total_store_revenue × 100` | **all** campaigns |
| Реклама | Доля рекламных расходов в выручке **с рекламных кампаний** | `total_ad_spend / attributed_ad_revenue × 100` | **all** campaigns |

7-day values = **ratio of sums**, never mean of daily/campaign percents. Zero denominator → null. Problem campaigns are classified **after** account aggregates.

Pack fields: `ad_spend_share_of_total_revenue_pct`, `ad_spend_share_of_attributed_revenue_pct` (+ legacy `account_drr_pct` = store share alias). Explicit money: `ad_spend(_7d)`, `revenue(_7d)` = store, `ad_revenue(_7d)` = attributed.

## 5. Changed areas (high level)

- `analytics`: `AdSpendShareOfAttributedRevenuePct`; ads block uses attributed denominator; Pulse uses store
- `dashboardfacts` / `dashboardhome`: dual money fields on summary; Pulse label/hint
- `pack` columns + dictionary for both shares
- Schema **v5** (no `recommendation`); prompt **v1_16**
- Validators: forbid recommendation; allow Pulse≠Ads; arithmetic / denominator checks
- Frontend: labels, tooltips, denominator note, chart `За 7 дней:`, no AI recommendation

## 6–8. Pack / prompt / validation

- Pack CSV: new share columns on `account_summary` / `account_daily`
- Prompt `unified_analyst_v1_16`: dual metrics, no recommendations, explicit revenue wording
- Schema `aivion_unified_analysis_v5`: narrative = headline + summary (+ checklist/limitations); no recommendation
- Codes: `advertising_recommendation_not_allowed`, share mismatch / mixed denominator / ambiguous semantics

## 9. Reconciliation (pub 41 / API)

| Metric | Spend | Denominator | Formula | Facts/API | AI text | UI |
|--------|------:|------------:|---------|----------:|--------:|----|
| Store share, day | 23 400 | 120 791.25 | 19.37% | Pulse 19.4% | (in period prose) | Pulse card |
| Store share, 7d | 163 800 | 975 389.79 | 16.79% | summary store 7d | **16,8%** | Pulse narrative |
| Attributed share, day | 23 400 | 25 500 | 91.76% | current_drr 91.76 | — | **91,8%** |
| Attributed share, 7d | 163 800 | 178 500 | 91.76% | average_drr 91.76 | **91,8%** | **91,8%** + chart |

`sum(campaign spend)` = account spend; `sum(attributed revenue)` = account attributed; store revenue from account metrics.

## 10–12. Run / AI / API

| Field | Value |
|-------|-------|
| run_id | **63** |
| publication_id | **41** |
| prompt | `unified_analyst_v1_16` |
| schema | `aivion_unified_analysis_v5` |
| pack_hash | `423dbe4556f8eafe…` |
| output_hash | `f07a7f206cfaea23…` |
| bundle_hash | `f4af3c47971edf12…` |
| response_id | `resp_08206219a9a9062b006a5e0d0c42f08197a85cb4c52042d7d3` |
| container | `cntr_6a5e0d0afde0819095d7611ad4f7f74a0713e3da2623b270` |
| file_id | `cfile_6a5e0d0be37c819184a22daa6418acc2` |
| CI calls | 7 |
| tokens | 17 996 in / 13 606 out |
| latency | ≈124 s |

AI explanation (pub 41): spend 163 800 ₽ = 16,8% of cabinet revenue (975 390 ₽); attributed revenue 178 500 ₽ → 91,8% spend share. **No recommendation field.**

## 13–15. Tooltip / browser / graph evidence

- Pulse label: «Доля рекламных расходов в общей выручке» + formula hint
- Ads note: Pulse compares to **cabinet** revenue; this block to **attributed** revenue
- Ads cards: attributed share **91,8%** today and for 7 days (not «Средний ДРР»)
- Chart: adaptive Y (~86–98%), label **«За 7 дней: 91,8%»**, flat 91,8% series on attributed revenue
- DOM: no `Средний ДРР`, no `Среднее:`, no recommendation testid

## 16. Test matrix

| Check | Result |
|-------|--------|
| `go test` unifiedanalysis / analytics / dashboardfacts / dashboardhome | PASS |
| `go vet` + build api/worker/devtools | PASS |
| Full `go test ./...` | PASS (config env test flaky if `DATABASE_URL` preset) |
| Frontend lint / tsc / unit tests / build | PASS |
| Playwright | (running / see CI log) |
| Real OpenAI unified run + publish | PASS (run 63 / pub 41) |
| Browser DOM acceptance | PASS |

## Intentional non-goals

- Do **not** force Pulse % == Ads %
- Do **not** filter campaigns out of account aggregates
- Old publications with recommendation remain readable; new schema omits the field
