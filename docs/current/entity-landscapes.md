# Entity Landscapes

Status: **Prompt №9.8 — Landscape Schema V5 + executable Profiles V5** (unchanged by Prompt №10). Discovery projects an allowlisted Detail subset; it does not change Landscape schema or profiles.

## Role

Deterministic typed projection of one sealed Factual Universe — “what factually happens,” never semantic interpretation.

## Production source

`landscape.FactualAPISource` reads only through `factualapi.API` (sealed context). Production Core uses **Profiles V5** (`DefaultCoreProfilesV5`); Detail uses **Profiles V5** (`DefaultDetailProfilesV5`). V1–V4 profile names and hashes remain immutable legacy. Profile conformance is enforced at build time.

| Block | Factual methods (examples) | Capability gate |
|-------|----------------------------|-----------------|
| GlobalCapabilities | `CapabilityBindings()` from sealed Universe | frozen at seal |
| DomainWindows | `ResolvePeriod` + domain coverage/lag | per domain |
| Account Sales | `GetAccountDemand` | `SALES.ACCOUNT_DEMAND` |
| Account Ads | `GetAdvertisingAccountSummary` | advertising caps |
| Account Finance | `GetFinanceAccountSummary` | finance caps |
| SKU Sales/Stock/Price/Search | BatchGet* | SKU domain caps |
| SKU Ads Traffic | `BatchGetSKUAdvertisingTraffic` | `ADVERTISING.SKU_TRAFFIC` |
| SKU Ads Attribution | `BatchGetSKUAdvertisingAttribution` | `ADVERTISING.SKU_ATTRIBUTION` |
| SKU Coverage | `BatchGetStockCoverage` | `STOCK.COVERAGE_7D` |
| SKU Inbound | `BatchGetSKUInbound` | `SUPPLY.*` |
| Campaign Traffic | `BatchGetCampaignTrafficPerformance` | `ADVERTISING.CAMPAIGN_TRAFFIC` |
| Campaign Attribution | `BatchGetCampaignAttributionPerformance` | `ADVERTISING.CAMPAIGN_ATTRIBUTION` |
| Detail extras | `FactualAPIDetailSource` (batch funnel/search/promo/targets) | selected entities only |

## Advertising Traffic ≠ Attribution (Prompt №9.6)

Landscape V4 never reuses a Traffic period for Attribution (or the reverse). DomainWindows emit two advertising rows:

- `Domain=advertising` `FactFamily=ADVERTISING.TRAFFIC`
- `Domain=advertising` `FactFamily=ADVERTISING.ATTRIBUTION`

Calculated DRR (`paid spend / attributed amount`) is a Landscape derivation only when Traffic and Attribution current (and previous) periods are identical. Otherwise `CalculatedDRR = nil` with limitation `DRR_PERIOD_MISMATCH`. Provider DRR remains a raw Attribution fact and is never averaged across campaigns.

Account and Category advertising blocks are split into Traffic and Attribution subblocks. Known zero aggregates serialize as `"0"`, not missing. Unknown demand is not counted as without-sales.

SupportingContext V4 preserves `WindowRef` + `Period` (period facts) or `WindowRef` + `AsOf` (snapshot facts) through truncation. Campaign SKU context is `BOUNDED_EXPLICIT_ASSIGNMENTS` / `CANONICAL_SKU_ID_ASC` / `CAMPAIGN_TARGETS_AS_OF`.

## Multi-family windows (Prompt №9.4)

Landscape lag/window resolution uses **source-native fact-family bindings** via `ResolveFactFamilyPeriodPair` — never ambiguous `DomainBinding("SEARCH")`. Search Seller Summary uses `SEARCH.SELLER_SUMMARY`; Query Detail uses `SEARCH.QUERY_DETAILS`. Account/Category/SKU sales periods anchor on `SALES.DEMAND` coverage, not analysis day alone when the profile specifies `FACT_FAMILY_COVERAGE_TO`.

**Capability fail-closed:** required profile blocks without a sealed capability binding produce `PROFILE_CAPABILITY_CONTRACT_VIOLATION` — no silent unbound reads.

## V3 Account derived blocks (Prompt №9.4)

After SKU/Category/Campaign load, Core derives: `account.assortment`, `account.stock`, `account.price`, `account.search`, `account.advertising`, `account.concentration`, `account.supply` (fallback from SKU inbound when direct summary unavailable).

## Query budgets (Prompt №9.4 gates)

Measured on real Postgres with pgx query tracing:

| Gate | Measured (full V4) | Ceiling |
|------|---------------------|---------|
| BuildCore 500 SKU | 67 SQL | ≤ 100 |
| BuildCore 2000 SKU | 97 SQL | ≤ Q500 + 50 |
| BuildDetail 15/40/15 | 25 SQL | ≤ 40 |

Batch reads chunk at `maxBatch=1000`; period DQ reconciliation runs once per period per batch call (not per SKU).

## SKU factual blocks (Prompt №9.2)

- **Search**: current + previous position/window on typed SKU Search block (`SearchPositionCurrent`, `SearchPositionPrevious`); comparable source-native periods only.
- **Price**: last observed change from factual price history (previous/current Money, change fields, `observed_at`).
- **Supply**: open inbound counts separate from canonical `OVERDUE` exposure (`OverdueSupplyCount` / presence from provider state — never `OpenInboundCount > 0`).
- **Detail Campaign context**: `ListSKUCampaignPerformance` — direct SKU advertising relationship only; bounded list with `total_count`, `returned_count`, `truncated`, deterministic sort (SKU paid spend DESC, stable Campaign ref).

## Error vs empty (Prompt №9.2 / №9.8)

Factual API failures on optional domains (Category Demand, Search, etc.) propagate as block-level `PARTIAL` or `UNAVAILABLE` — never silent empty maps that become `READY_EMPTY`. Known genuinely empty data remains `READY_EMPTY`.

Prompt №9.8: `BatchLimited` SKU Demand is retained. UNKNOWN / `MISSING_DAILY_FACT` periods become Landscape `PARTIAL` with compact limitations — never `READY_EMPTY`. Account and Category Sales read `FactMeta.Readiness` and limitations even when the Go error is nil. Known `ANALYSIS_DAY` facts are retained when `CURRENT_7D` is unknown.

Eligibility: `LANDSCAPE_ELIGIBILITY_V1` (SKU OR-rules; Campaign active/spend/attribution; leaf Category + UNMAPPED reconciliation bucket).

## V4 Profile matrix (Prompt №9.6)

| Entity | Block | Core/Detail | Capability | FactFamily | Method | Period | Derived formula | Supporting policy |
|--------|-------|-------------|------------|------------|--------|--------|-----------------|-------------------|
| Account | account.advertising.traffic | Core | ADVERTISING.CAMPAIGN_TRAFFIC | ADVERTISING.TRAFFIC | derived | CURRENT_7D/PREVIOUS_7D source-native | account_ads_traffic_from_campaign_v4 | — |
| Account | account.advertising.attribution | Core | ADVERTISING.CAMPAIGN_ATTRIBUTION | ADVERTISING.ATTRIBUTION | derived | CURRENT_7D/PREVIOUS_7D source-native | account_ads_attribution_from_campaign_v4 | — |
| Category | category.advertising.traffic | Core | ADVERTISING.SKU_TRAFFIC | ADVERTISING.TRAFFIC | derived | CURRENT_7D/PREVIOUS_7D source-native | category_ads_traffic_from_sku_v4 | — |
| Category | category.advertising.attribution | Core | ADVERTISING.SKU_ATTRIBUTION | ADVERTISING.ATTRIBUTION | derived | CURRENT_7D/PREVIOUS_7D source-native | category_ads_attribution_from_sku_v4 | — |
| Category | category.funnel | Detail | FUNNEL.CATEGORY_ADDITIVE | FUNNEL.DAILY_ADDITIVE | BatchGetCategoryFunnelPeriod | CURRENT_7D/PREVIOUS_7D | — | — |
| Category | category.search | Detail | SEARCH.CATEGORY_SUMMARY | SEARCH.SELLER_SUMMARY | BatchGetCategorySearchDistribution | CURRENT_7D/PREVIOUS_7D | nearest_rank_quantile_v1 | — |
| Category | category.promotions | Detail | PROMOTIONS.MEMBERSHIP | PROMOTIONS.MEMBERSHIP | BatchGetCategoryPromotionSummary | snapshot | — | as-of cutoff |
| SKU | sku.advertising.traffic | Core | ADVERTISING.SKU_TRAFFIC | ADVERTISING.TRAFFIC | BatchGetSKUAdvertisingTraffic | CURRENT_7D/PREVIOUS_7D | CTR/CPC/CPM | — |
| SKU | sku.advertising.attribution | Core | ADVERTISING.SKU_ATTRIBUTION | ADVERTISING.ATTRIBUTION | BatchGetSKUAdvertisingAttribution | CURRENT_7D/PREVIOUS_7D | CalculatedDRR iff periods match | — |
| SKU | sku.funnel | Detail | FUNNEL.SKU | FUNNEL.DAILY_ADDITIVE | BatchGetSKUFunnelPeriod | CURRENT_7D/PREVIOUS_7D | six metrics × 2 periods | — |
| SKU | sku.detail.campaign_context | Detail | ADVERTISING.SKU_TRAFFIC | ADVERTISING.TRAFFIC | BatchListSKUCampaignPerformance | CURRENT_7D Traffic | — | TOP_N_BY_PAID_SPEND K=5 |
| SKU | sku.detail.promotion_context | Detail | PROMOTIONS.MEMBERSHIP | PROMOTIONS.MEMBERSHIP | BatchGetSKUPromotions | snapshot | — | PROMOTIONS_AS_OF |
| SKU | sku.detail.query_context | Detail | SEARCH.QUERY_DETAILS | SEARCH.QUERY_DETAILS | BatchGetSKUQueryDetails | CURRENT_7D Query Details | — | SEARCH_QUERY_DETAILS_CURRENT |
| Campaign | campaign.traffic | Core | ADVERTISING.CAMPAIGN_TRAFFIC | ADVERTISING.TRAFFIC | BatchGetCampaignTrafficPerformance | CURRENT_7D/PREVIOUS_7D | CPC/CPM/CTR | — |
| Campaign | campaign.attribution | Core | ADVERTISING.CAMPAIGN_ATTRIBUTION | ADVERTISING.ATTRIBUTION | BatchGetCampaignAttributionPerformance | CURRENT_7D/PREVIOUS_7D | CalculatedDRR iff periods match | optional, no Traffic requirement |
| Campaign | campaign.targeting | Core | ADVERTISING.CAMPAIGN_TRAFFIC | ADVERTISING.TRAFFIC | BatchGetCampaignTargetingSummary | CURRENT_7D Traffic | campaign-specific observed SKU | — |
| Campaign | campaign.detail.sku_context | Detail | ADVERTISING.CAMPAIGN_CONFIG | ADVERTISING.TRAFFIC | BatchListCampaignTargetSKUs | snapshot | — | CANONICAL_SKU_ID_ASC / BOUNDED_EXPLICIT_ASSIGNMENTS / CAMPAIGN_TARGETS_AS_OF |

## Artifacts

`LANDSCAPE_CORE_SET` / `LANDSCAPE_DETAIL_SET` — rebuildable; no mutable Landscape SoT tables.
