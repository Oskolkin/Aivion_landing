# Seller 1 Full Reseed — Current Repository Baseline

**Date:** 2026-07-21  
**Prompt / schema / pack (target after reseed):** `unified_analyst_v1_20` / `aivion_unified_analysis_v7` (→ v8 for evidence) / `aivion_analysis_pack_v1`  
**Status:** Pre-implementation baseline. Destructive reset must not run until sections 1–7 + tests pass.

---

## 1. Confirmed root causes

| # | Claim | Verdict | Evidence |
|---|-------|---------|----------|
| 1 | `mvp_cleanup.go` cleans incomplete seller-scoped tables | **CONFIRMED** | `cleanupMVPSellerData` deletes 28 tables only |
| 2 | `reset_demo_seller_data.go` duplicates cleanup and swallows errors | **CONFIRMED** | `if err == nil { deleted = append(...) }` |
| 3 | `dev-seed-mvp` does not call `AssertDevSeedAllowed()` on write-run | **CONFIRMED** | `SeedMVP` / `main.go` never call guard |
| 4 | `mvp_metrics.go` patches `daily_sku_metrics` after rebuild | **CONFIRMED** | `RebuildMVPAnalytics` → four `PatchMVP*` |
| 5 | Patch functions mutate derived SKU metrics without reconciliation | **CONFIRMED** | No account/sales re-sync |
| 6 | Key SKU uses absolute revenues unrelated to account | **CONFIRMED** | Hard-coded `TargetRevenue7D` |
| 7 | Long-tail: `revenue > 0` with `orders_count = 0` | **CONFIRMED** | `mvpKeySkuLongTailOrdersPerDay=0`, revenue `80` |
| 8 | `SeedOzonEnrichment` rewrites `ad_metrics_daily` after `SeedMVPAds` | **CONFIRMED** | `seedAdMetricsDaily` upserts |
| 9 | Stock analytics independent of `stocks` | **CONFIRMED** | `stockScenarioForIndex` / `stockAnalyticsValues` |
| 10 | Account vs SKU orders from different sources | **CONFIRMED** | Account ← `orders`; SKU ← `sales` rows |
| 11 | Dashboard uses account revenue + SKU-summed orders | **CONFIRMED** | `BuildDashboardMetricsFromSnapshot` |
| 12 | Seed / Dashboard facts validators lack full reconciliation | **CONFIRMED** | Presence/DRR only |
| 13 | Unified Action adapter always saves empty evidence/fact IDs | **CONFIRMED** | Hardcoded `[]` in adapter |

---

## 2. Files and functions

### Incomplete cleanup

| File | Function | Problem |
|------|----------|---------|
| `backend/internal/devseed/mvp_cleanup.go` | `cleanupMVPSellerData` | 28 tables; misses AI/actions/enrichment/unified/chat_attachments |
| `backend/cmd/devtools/reset_demo_seller_data.go` | `runResetDemoSellerData` | Duplicate SQL list; swallows `Exec` errors; only adds 3 AI summary tables |

### Guard missing on MVP seed write path

| File | Function | Notes |
|------|----------|-------|
| `backend/internal/devseed/dev_seed_guard.go` | `AssertDevSeedAllowed` | Exists; used by enrichment/ads/actions/reset, **not** by `SeedMVP` |
| `backend/cmd/dev-seed-mvp/main.go` | `main` → `SeedMVP` | No guard before write |
| `backend/internal/devseed/mvp_seed.go` | `SeedMVP` | No guard |

### Post-rebuild SKU patches

| File | Function |
|------|----------|
| `backend/internal/devseed/mvp_metrics.go` | `RebuildMVPAnalytics` |
| `backend/internal/devseed/mvp_sku_focus_order_drop.go` | `PatchMVPOrderDropDemoMetrics` |
| `backend/internal/devseed/mvp_sku_focus_stock_risk.go` | `PatchMVPStockRiskDemoMetrics` |
| `backend/internal/devseed/mvp_sku_focus_no_sales.go` | `PatchMVPNoSalesDemoMetrics` |
| `backend/internal/devseed/mvp_sku_focus_key_skus.go` | `PatchMVPKeySkuDemoMetrics`, `suppressMVPGenericSkuLongTail` |

### Ads / stock dual writers

| File | Function |
|------|----------|
| `backend/internal/devseed/mvp_ads.go` | `SeedMVPAds` — canonical ads for MVP |
| `backend/internal/devseed/ozon_enrichment_seed.go` | `seedAdMetricsDaily`, `seedStockAnalyticsSnapshots` |
| `backend/internal/devseed/ozon_enrichment_seed_generators.go` | `stockAnalyticsValues`, `stockScenarioForIndex` |

### Dashboard orders semantics

| File | Function |
|------|----------|
| `backend/internal/analytics/dashboard_from_snapshot.go` | `BuildDashboardMetricsFromSnapshot` — revenue from account, orders from SKU sum |
| `backend/sql/queries/daily_account_metrics.sql` | Account orders from `orders` table |
| `backend/sql/queries/daily_sku_metrics.sql` | SKU orders from `sales` row counts |

### Empty action evidence

| File | Function |
|------|----------|
| `backend/internal/unifiedanalysis/publication/adapter/actions.go` | Persist with `EvidenceJSON: []`, `EvidenceFactIDs: []`, `SourceCandidateIDs: []` |
| Unified contract | Evidence fact IDs intentionally excluded from AI schema v7 |

---

## 3. Current seed execution order (`SeedMVP`)

1. `ValidateMVPSeedOptions`
2. Begin tx → `ResolveSellerForMVP` → `EnsureAdminUserIfRequested`
3. If `opts.Reset`: `cleanupMVPSellerData` (incomplete)
4. `SeedOzonConnectionMock`
5. `SeedMVPCommerce` → products, stocks, orders, sales
6. `SeedMVPAds` → campaigns, `ad_metrics_daily`, SKU links
7. `SeedMVPPricing`
8. `SeedSellerBillingSupportStub`
9. `SeedMVPSyncArtifacts`
10. `SeedMVPAdminMetadataInTx`
11. **Commit source tx**
12. `PatchLegacyPicsumProductImages`
13. `RebuildMVPAnalytics` → account rebuild → SKU rebuild → trim → **PatchMVP\*** (forbidden pattern)
14. `SeedMVPProductAnalytics`
15. `SeedMVPAdminPostMetricsLog`
16. Summary loaders

**Separate CLI (typical after MVP):** `SeedOzonEnrichment` → may rewrite `ad_metrics_daily` and invent stock analytics.

**Failure mode:** Source commits before metrics/enrichment/validation; partial failure leaves inconsistent seller 1.

---

## 4. Cleanup coverage gap

### Cleaned today (`cleanupMVPSellerData`)

`recommendation_run_diagnostics`, `recommendation_feedback`, `recommendation_alert_links`, `recommendation_runs`, `recommendations`, `chat_feedback`, `chat_traces`, `chat_messages`, `chat_sessions`, `alert_runs`, `alerts`, `daily_sku_metrics`, `daily_account_metrics`, `product_analytics_daily`, `ad_metrics_daily`, `ad_campaign_skus`, `ad_campaigns`, `sku_effective_constraints`, `pricing_constraint_rules`, `sales`, `orders`, `stocks`, `products`, `import_jobs`, `sync_jobs`, `sync_cursors`, `raw_payloads`, `ozon_connections`, `admin_action_logs`, `seller_billing_state`

### Extra only in `reset_demo_seller_data`

`ai_dashboard_narratives`, `ai_daily_summaries`, `ai_daily_summary_runs`

### Not cleaned (must be covered by unified reset)

**Unified / AI:**  
`seller_current_unified_publications`, `seller_current_ai_snapshots`, `unified_analysis_publication_audit`, `unified_analysis_publications`, `ai_pipeline_dispatches`, `ai_pipeline_runs`, `ai_usage_events`, `ai_dashboard_data_snapshots`, `ai_dashboard_narratives`, `ai_daily_summaries`, `ai_daily_summary_runs`  
**Via parent IDs:** `unified_publication_action_map`, `ai_pipeline_artifacts`

**Actions:**  
`action_user_states`, `action_entity_groups`, `action_generation_category_runs`, `action_items`, `action_generation_runs`

**Chat / enrichment:**  
`chat_attachments`, `ozon_api_capabilities`, `ozon_enrichment_sync_runs`, `ozon_product_info_snapshots`, `ozon_related_sku_errors`, `ozon_related_skus`, `stock_analytics_snapshots`, `ad_campaign_objects`

**Cyclic FK note:**  
`unified_analysis_publications.run_id` ↔ `ai_pipeline_runs.unified_publication_id` — must NULL pointer before delete.

### Intentionally preserved

`users`, `seller_accounts`, user↔seller links, auth/sessions, `seller_analysis_settings`

---

## 5. Missing reconciliation rules (current validators)

Neither `ValidateMVPSeededSeller` nor `dashboardfacts/validate.go` enforce:

| Domain | Missing rule |
|--------|--------------|
| Account ↔ source | revenue = Σ sales; orders = COUNT(orders); units = Σ sales units |
| Account ↔ SKU | Σ SKU revenue/orders/units = account; no SKU > account; no revenue with zero orders/units |
| Category | one mapping per SKU; unmapped=0; category sums = SKU sums |
| Advertising | campaign spend/revenue = account; no-orders = spend>0 AND orders=0; weekly ratio diversity |
| Stocks | raw stocks aggregate = daily SKU stock = stock analytics = pack = dashboard |
| Funnel | stage monotonicity; conversions from stages; orders aligned with account |
| Identity | no other seller rows; no dates after anchor; no duplicates; no negatives; CTR/conversion ≤ 100% |
| Dashboard facts | full current-day account/SKU/category/stock/ad/funnel reconciliation |
| Pre-OpenAI | no gate on pack snapshot before StatusPackReady |
| Actions | evidence fact IDs required and grounded; completion baseline/target validated |

---

## 6. Forbidden patterns to eliminate

```go
// reset_demo_seller_data.go — swallow errors
if _, err := tx.Exec(...); err == nil {
    deleted = append(deleted, step.name)
}

// RebuildMVPAnalytics — patch derived metrics
PatchMVPOrderDropDemoMetrics(...)
PatchMVPStockRiskDemoMetrics(...)
PatchMVPNoSalesDemoMetrics(...)
PatchMVPKeySkuDemoMetrics(...)

// Long-tail invariant violation
orders_count = 0 AND revenue = 80

// Adapter
EvidenceFactIDs: []byte("[]")
```

---

## 7. Target end state (post-implementation)

1. Single `ResetSellerTestData` implementation used by MVP cleanup, `--reset`, and `reset-demo-seller-data`.
2. Source-first scenario plan → write source → one metrics rebuild → enrichment reuses ads/stocks → reconciliation.
3. Pre-OpenAI reconciliation gate on pack snapshot.
4. Action evidence + completion conditions grounded.
5. Full seller-1 reseed + new Unified run + audit ZIP with no `NOT_CHECKED`.
