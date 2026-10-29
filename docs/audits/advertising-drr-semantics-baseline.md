# Advertising DRR / spend-share semantics — baseline (pre-change)

Date: 2026-07-20  
Seller: 1  
Metric date: 2026-07-08  
Source publication: 34 (run 55), Analysis Pack hash `ddae51bcc4cabf8f…`

## Root cause of value divergence (historical)

Two different denominators exist in the data plane:

| Denominator | Source | Example (2–8 Jul) |
|-------------|--------|-------------------|
| Cabinet / store revenue | `daily_account_metrics.revenue` | 7d sum ≈ **975 390 ₽** |
| Attributed ad revenue | `ad_metrics_daily.revenue` (campaign attributed) | 7d sum = **178 500 ₽** |

Same numerator (all-campaign spend 7d = **163 800 ₽**) yields:

- spend ÷ store ≈ **16,8%**
- spend ÷ attributed ≈ **91,8%**

After the prior “unify DRR” change, **both** Pulse and Ad-efficiency UI used the **store** denominator (~19,4% today / ~16,8% for 7d). The pack CSV still exposed `account_drr_pct=91.76` from attributed revenue, and AI sometimes mixed spend + attributed revenue while quoting the store share — arithmetic inconsistency.

This baseline records **current** (pre-split) behaviour. The follow-up change intentionally **separates** the two business metrics again with explicit names.

---

## Baseline table (current code / pub 34)

| Field | Numerator | Denominator | Period | Campaign scope | Current value |
|-------|-----------|-------------|--------|----------------|---------------|
| Pulse: current ad share | all-campaign spend day | **store** revenue day | 2026-07-08 | all campaigns | **19,4%** (19.37) |
| Pulse: daily series | spend day | **store** revenue day | 2026-07-02…08 | all | 20.06 … 19.37 |
| Pulse: 7d (spark only; no separate KPI) | — | — | — | — | series only |
| Ads: `current_drr_pct` | spend day | **store** revenue day | 2026-07-08 | all | **19.37%** |
| Ads: `trend_7d[].drr_pct` | spend day | **store** revenue (`trend.revenue`) | daily | all | same as pulse series |
| Ads: `average_drr_7d_pct` | Σ spend | Σ **store** revenue | 7d sum-of-sums | all | **16.79%** |
| Ads AI narrative (run 55) | mentions 163 800 spend + 178 500 “доход” + 16,8% | **mixed** | 7d | — | inconsistent |

---

## Number dictionary (seller 1, 2026-07-08 window)

| Token | Meaning | Value |
|-------|---------|------:|
| `163 800 ₽` | `ad_spend_7d` — sum of **all** campaign spend 2–8 Jul | 163 800 |
| `178 500 ₽` | `ad_revenue_7d` — sum of **attributed** campaign revenue 2–8 Jul | 178 500 |
| Store revenue 7d | `revenue_7d` from account metrics | ≈ 975 390 |
| Store revenue 8 Jul | account revenue that day | 120 791.25 |
| Attributed revenue 8 Jul | sum campaign attributed revenue | 25 500 |
| Spend 8 Jul | all-campaign spend | 23 400 |
| **16,8%** | 163 800 / 975 390 — share of **store** revenue (7d) | ≈ 16.79 |
| **19,4%** | 23 400 / 120 791 — share of **store** revenue (day) | ≈ 19.37 |
| **91,8%** | 163 800 / 178 500 — share of **attributed** ad revenue (7d); also daily 23400/25500 | ≈ 91.76 |

Daily pack check (Jul 2–8): spend and attributed revenue are flat (23 400 / 25 500 → 91.76% every day); store revenue varies → store-share varies 12.2–20.1%.

---

## Path map

```
AnalysisDataSnapshot (AccountMetrics + AdMetrics + Campaigns)
  → analytics.AccountDRRPct(spend, storeRevenue)          // pulse + ads today
  → analytics.AccountDRRPeriodPct / BuildAccountDRRDailySeries
  → dashboardfacts.seller_pulse (KPI key "drr")
  → dashboardfacts.ads / AdEfficiencyFacts
  → pack account_summary/account_daily (ad_spend, ad_revenue, revenue, account_drr_pct*)
  → AI structured output ad_efficiency.narrative
  → publication adapter + frozen snapshot
  → Dashboard API → SellerPulseSection + AdEfficiencyModule
```

\*Pack `account_drr_pct` currently encodes **attributed** share (91.76) while dashboard facts encode **store** share — latent dictionary/UI mismatch.

### Campaign scope

Account aggregates sum **all** campaigns in `ad_metrics_daily` for the seller/date (no filter by high-DRR / no-orders / AI selection). Problem lists are post-aggregate classification.

---

## Target semantics (post-change)

| Block | Metric | Formula |
|-------|--------|---------|
| Пульс продавца | Доля рекламных расходов в **общей** выручке | spend / **total_store_revenue** × 100 |
| Реклама | Доля рекламных расходов в выручке **с рекламных кампаний** | spend / **attributed_ad_revenue** × 100 |

7d values = ratio of sums, never mean of daily %.
