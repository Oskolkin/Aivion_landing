# Dashboard field ownership (Unified Analysis)

**Purpose:** Merge policy and test oracle for Dashboard facts vs AI analysis.  
**Owners:** `FACT` / `DERIVED_FACT` = backend (immutable Dashboard Facts from AnalysisDataSnapshot); `AI_ANALYSIS` = model; `PRESENTATION` = formatting / UI-only.

**Rule:** Backend facts always win for factual fields. AI never overwrites FACT/DERIVED_FACT. Missing facts → null / not_enough_data / source_not_available — never silent AI numbers.

---

## Block ownership summary

| Block | Facts owner | Analysis owner |
| ----- | ----------- | -------------- |
| seller_pulse.kpis (revenue, orders, average_order_value, ad_spend_share_total_revenue) | backend | — |
| seller_pulse.trend_7d | backend | — |
| seller_pulse.sparklines / deltas | backend | — |
| seller_pulse.narrative | — | AI |
| seller_pulse.important_notes / quick_actions | — | AI |
| dashboard_metrics.account / skus | backend | — |
| stock_coverage.segments / totals / status | backend | — |
| stock_coverage.narrative | — | AI |
| ad_efficiency.summary / points / campaigns | backend | — |
| ad_efficiency.narrative | — | AI |
| sales_funnel.stages / conversions / deltas | backend | — |
| funnel_analysis.ai_analysis | — | AI |
| sku_focus factual metrics (name, offer_id, revenue, orders, stock, coverage) | backend | — |
| sku_focus problem / reason / priority selection | — | AI (entity pick) + backend (facts join) |
| account_summary.headline / executive_summary / improvement_summary | — | AI |
| account_summary.risk_drivers | — | AI |
| what_changed | — | AI |
| category_summaries (analytical text) | — | AI |
| risk_distribution (if from AI groups) | hybrid* | AI labels; counts preferably backend when available |
| actions / steps / expected_effect | — | AI |
| date / freshness metadata | backend | — |
| data_quality / limitations | backend + AI limitations list | AI may add notes; backend owns stale/historical semantics |

\* Risk distribution from open alerts is backend when available; Unified v1 may omit alert-backed risks if not in snapshot.

---

## Field table

| Field | Owner type | Source | Calculation | AI allowed to overwrite |
| ----- | ---------- | ------ | ----------- | ----------------------- |
| dashboard_metrics.seller_account_id | FACT | snapshot | identity | no |
| dashboard_metrics.as_of_date | FACT | snapshot.MetricDate | date format | no |
| dashboard_metrics.account.revenue_today | FACT | AccountMetrics[metric_date] | sum/day | no |
| dashboard_metrics.account.revenue_yesterday | FACT | AccountMetrics[metric_date-1] | day | no |
| dashboard_metrics.account.revenue_last_7d | DERIVED_FACT | AccountMetrics window | sum 7d | no |
| dashboard_metrics.account.revenue_day_to_day_delta | DERIVED_FACT | today vs yesterday | abs + pct | no |
| dashboard_metrics.account.revenue_week_to_week_delta | DERIVED_FACT | last7 vs prev7 | abs + pct | no |
| dashboard_metrics.account.orders_today / yesterday | FACT | AccountMetrics | day | no |
| dashboard_metrics.account.returns_today / cancels_today | FACT | AccountMetrics | day | no |
| dashboard_metrics.account.average_order_value | DERIVED_FACT | revenue/orders | AOV | no |
| dashboard_metrics.account.account_ad_spend_to_revenue | DERIVED_FACT | AdMetrics + AccountMetrics | spend/revenue | no |
| dashboard_metrics.skus[*].revenue / orders / stock / days_of_cover | FACT / DERIVED_FACT | SKUMetrics + StockSnapshots | pack/analytics canonical | no |
| dashboard_metrics.skus[*].share_of_revenue | DERIVED_FACT | SKU / account | ratio | no |
| seller_pulse.kpis[key=revenue] | PRESENTATION of FACT | account.revenue_today | formatMoney + delta + sparkline | no |
| seller_pulse.kpis[key=orders] | PRESENTATION of FACT | account.orders_today | formatInt + delta + sparkline | no |
| seller_pulse.kpis[key=average_order_value] | DERIVED_FACT | account.revenue_today / orders_today | AOV (null if orders=0) | no |
| seller_pulse.kpis[key=ad_spend_share_total_revenue] | DERIVED_FACT | ad spend / store revenue | % + sparkline (not attributed DRR) | no |
| seller_pulse.kpis[key=risks] | FACT (optional) | alerts if available; else omit | count | no |
| seller_pulse.trend_7d[].date/revenue/orders | FACT | AccountMetrics observed days only | no zero-fill for missing days | no |
| seller_pulse.narrative.* | AI_ANALYSIS | AI seller_pulse.narrative | — | yes (AI-owned) |
| stock_coverage.segments | DERIVED_FACT | SKU days_of_cover buckets | StockCoverageBucketKey | no |
| stock_coverage.total_skus | FACT | SKU universe | count | no |
| stock_coverage.narrative.* | AI_ANALYSIS | AI | — | yes |
| ad_efficiency.summary.* | DERIVED_FACT | AdMetrics + Campaigns | canonical ad efficiency | no |
| ad_efficiency.points | DERIVED_FACT | daily ad series | observed days | no |
| ad_efficiency.narrative.* | AI_ANALYSIS | AI | — | yes |
| sales_funnel.stages | DERIVED_FACT | FunnelRows | buildFunnelStages | no |
| sales_funnel.status | DERIVED_FACT | coverage | ok / not_enough_data / source_not_available | no |
| funnel_analysis.ai_analysis.* | AI_ANALYSIS | AI | — | yes |
| sku_focus.*.sku / offer_id / product_name | FACT | Products index | join by entity id | no |
| sku_focus.*.revenue / orders / stock / coverage | FACT | SKUFactsIndex | join | no |
| sku_focus.*.problem / reason / priority | AI_ANALYSIS | AI selection + text | unknown SKU → reject/drop per validation policy | text yes; ids must exist in facts |
| account_summary.headline | AI_ANALYSIS | AI | — | yes |
| account_summary.executive_summary | AI_ANALYSIS | AI | — | yes |
| account_summary.improvement_summary | AI_ANALYSIS | AI | — | yes |
| account_summary.health_score / severity | DERIVED_FACT | backend ComputeHealthScoreBreakdown(risk_drivers) at publish | AI may propose; published score is backend | no (AI text only) |
| account_summary.health_score_breakdown | DERIVED_FACT | same formula | always exposed | no |
| account_summary.risk_drivers | AI_ANALYSIS | AI | selection + copy; score inputs | text yes |
| top_action_ids / recommended_action_ids | AI_ANALYSIS | AI actions | must resolve | yes |
| actions[*] | AI_ANALYSIS | AI | — | yes |
| metric_date / requested_date / effective_metric_date | FACT | date resolution | — | no |
| latest_complete_data_date | FACT | date resolution | — | no |
| source_freshness_lag_days | DERIVED_FACT | today − latest_complete | — | no |
| analysis_age_days | DERIVED_FACT | today − effective_metric_date | — | no |
| historical_analysis | DERIVED_FACT | date_mode=fixed | — | no |
| data_is_stale | DERIVED_FACT | source freshness threshold (not analysis age alone) | — | no |
| pack_id / pack_hash / facts_hash | FACT | pack + facts artifacts | sha256 | no |

---

## Merge policy (publication)

```text
for each field:
  if owner in {FACT, DERIVED_FACT, PRESENTATION-of-FACT}:
      use backend facts (never AI number fallback)
  if owner == AI_ANALYSIS:
      use AI (required when analysis_outcome requires narrative/actions)
  if AI factual arrays empty but facts present:
      publish facts + AI narrative → valid
  if facts missing for covered source:
      bundle invalid → do not publish
```

---

## Identity

```text
seller_id + metric_date + pack_id + pack_hash + facts_hash
```

Facts must be built from the same `AnalysisDataSnapshot` instance used for the Analysis Pack.
