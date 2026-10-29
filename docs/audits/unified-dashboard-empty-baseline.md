# Unified Dashboard empty baseline (forensic)

**Date:** 2026-07-17  
**Scope:** seller_account_id=1, pipeline_run_id=406, publication_id=272, metric_date=2026-07-08  
**Method:** live PostgreSQL + `/api/v1/dashboard/home` + `/api/v1/actions` + browser at `http://localhost:3000/app/dashboard`  
**Values below are observations only** — not hardcoded into production code.

---

## 1. Pointers and publication

| Item | Value |
|------|-------|
| `seller_current_unified_publications` | publication_id=272 |
| `seller_current_ai_snapshots` | pipeline_run_id=406 |
| publication status / visibility / mode | published / visible / unified |
| analysis_outcome | material_actions_found |
| pack_hash (prefix) | `9947769e8110a81e…` |
| pack_id | `pack:1:2026-07-08:9947769e8110a81e` |
| facts artifact `unified_dashboard_facts_v1` | **absent** |

---

## 2. Content counts

| Metric | Value |
|--------|------:|
| output_kpis | 3 |
| output_trend_days | 7 |
| output_headline (len) | 59 (present) |
| output_stock_segments | 0 |
| output_funnel_stages | 5 |
| output_ad_points | 0 |
| output_sku_focus items (AI arrays) | key_skus+no_sales present; order_drop/stock_risk empty |
| output_actions | 1 |
| frozen_kpis | 3 |
| frozen_trend_days | 7 |
| frozen_stock_segments | 0 |
| frozen_funnel_stages | 5 |
| frozen_ad_points | 0 |
| frozen_sku_focus_items | projected from AI (no_sales=16, key_skus=10 in API) |
| frozen `dashboard_metrics.account.*` | **all zeros** (shell only: seller_id + as_of_date) |
| api_kpis | 3 |
| api_trend_days | 7 |
| api_headline | present (`narrative_source=ai`, `prompt_version=unified_publication_adapter_v1`) |
| api_actions | 1 |
| api actions `publication_id` | **missing** (only `analysis_source=unified`, `pipeline_run_id=406`) |
| api status | `ready` |

### AI SellerPulse KPI keys (output / frozen / API)

```text
orders_7d = 706
revenue_7d = 1051106.58
avg_order_value_7d = 1488.82
```

### Frontend-expected SellerPulse KPI keys (legacy contract)

```text
revenue, orders, ctr, drr  (+ optional risks)
```

---

## 3. DB vs AI numbers (same metric_date window)

`daily_account_metrics` seller=1, 2026-07-02..2026-07-08:

| Source | revenue 7d | orders 7d |
|--------|------------:|----------:|
| DB sum | 1051106.58 | 706 |
| AI KPI | 1051106.58 | 706 |

Daily trend **does not match** DB (AI invents/rewrites day points):

| Date | DB revenue | AI trend revenue |
|------|------------:|-----------------:|
| 2026-07-02 | 121126.53 | 136647.13 |
| 2026-07-03 | 116887.77 | 126311.31 |

Conclusion: even when totals coincide, day-level facts are not authoritative when taken from AI.

---

## 4. API / UI symptoms

### API (`GET /api/v1/dashboard/home`)

- `analysis_source=unified`, `publication_id=272`, `metric_date=2026-07-08`
- SellerPulse KPIs present but under **wrong keys** for UI (`revenue_7d` etc.)
- Funnel stages present under `ai_diagnostics.sales_funnel` (5)
- Stock: `total_skus=80`, **segments=[]**
- Ad: summary numbers present, **points=[]**, spend/revenue “today” = 0
- `data_is_stale=true`, `data_freshness_lag_days=9`, note: `Данные не обновлялись 9 дн.`
- `latest_complete_data_date=2026-07-17` while analysis is fixed to 2026-07-08

### Browser (`/app/dashboard`)

Observed:

- Banner: «Часть данных ещё загружается» + «Данные не обновлялись 9 дн.»
- Seller Pulse narrative text visible
- KPI cards **Выручка / Заказы / CTR / ДРР** → «Метрика появится после синхронизации» / «Недостаточно данных»
- Воронка / Покрытие остатков / Реклама → «Не удалось загрузить блок»
- Top action present (Actions path works for the run)

Screenshot evidence: `baseline-dashboard-406.png` (captured 2026-07-17 during this audit).

---

## 5. Exact loss location

| Layer | What it contained | Verdict |
|-------|-------------------|---------|
| Analysis Pack / snapshot data | Account metrics exist in DB for metric_date; pack built | **Not the loss point** |
| AI output | Headline, KPIs (wrong keys), trend (wrong daily values), funnel stages; empty stock segments / ad points | Partial / unreliable facts |
| **Publication adapter `ProjectDashboard`** | Copies AI widgets; sets `DashboardMetrics` to **seller_id + as_of_date only** | **PRIMARY LOSS** |
| Frozen storage | Mirrors adapter output (zeros in `dashboard_metrics`, AI KPI keys) | Stores the loss; does not invent it |
| Read path (`BuildHome` + compat pointer) | Applies frozen; forces `StatusReady` for unified narrative | Propagates empty/wrong KPIs; masks emptiness with ready |
| Frontend | Resolves cards by keys `revenue`/`orders`/`ctr`/`drr` | Shows placeholders when keys mismatch |

**Root cause (precise):**  
Data is lost / never produced at **`backend/internal/unifiedanalysis/publication/adapter/dashboard.go` → `ProjectDashboard`**: no deterministic Dashboard facts from `AnalysisDataSnapshot`; factual SellerPulse KPI contract (`revenue`/`orders`/`ctr`/`drr`) is not rebuilt; AI numeric arrays are trusted. Empty stock/ad factual arrays still publish. Output validator does not require factual KPI keys or non-empty factual blocks. Frontend therefore renders an “empty” dashboard while status/publication/actions look successful.

Secondary defects confirmed in the same state:

1. No `unified_dashboard_facts_v1` artifact.
2. Fixed-date analysis incorrectly marked stale (`analysis_age` confused with `source_freshness`).
3. Actions API omits `publication_id`.
4. Authoritative unified selector exists but Dashboard/Actions still resolve via `seller_current_ai_snapshots` / latest publishable run heuristics.

---

## 6. Code anchors

```go
// ProjectDashboard — DashboardMetrics shell only; widgets from AI
DashboardMetrics: analytics.DashboardMetricsDTO{
    SellerAccountID: sellerAccountID,
    AsOfDate:        metricDate,
},
SellerPulse: projectSellerPulse(dash.SellerPulse),
```

```go
// Legacy SellerPulse keys (what UI expects)
Key: "revenue" / "orders" / "ctr" / "drr"
```

```go
// enforceAIOnlyGenerationRequired — unified narrative → StatusReady
```

---

## 7. Baseline implication for fix

Must introduce immutable **Dashboard Facts** from the same snapshot as the pack, merge with AI analytical layer only, validate the **publication bundle** before pointer update, and serve Dashboard + Actions from one authoritative publication source.
