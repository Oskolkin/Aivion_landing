# Stock, Price and Advertising canonical boundary

Status: **NORMATIVE** for Prompt №6.

Stock, Price and Advertising are separate canonical domains. They share Collection provenance, seller-scoped private OAuth, immutable payload storage, authoritative-head protection and frozen Factual Read Contexts; they do not share a generic metric or EAV model.

## Stock

Stock is a point-in-time snapshot over a SKU universe frozen at collection start. Provider locations and inventory pools are stable identities with observed versions. Quantities retain provider field names until a source contract explicitly accepts a mapping. `valid_stock_count` is not sellable stock. Only pools marked `VERIFIED_INDEPENDENT` may contribute to the canonical known subtotal; overlap-unverified positions remain visible and excluded. Missing is never zero. Provider ADS/IDC/turnover fields are observations, not reconstructed demand.

## Price

Price is observed provider state, not an event inferred from polling. The primary contract is `POST /v5/product/info/prices`; each new authoritative run starts at an empty cursor and freezes its catalog universe. Money is exact decimal plus ISO currency. Zero semantics are field-specific: a zero current price is invalid, while zero in an accepted reference-price field may mean not set. Marketing fields remain separate until their customer/seller meaning is accepted. Optional premium enrichment cannot block the core price capability.

## Advertising

Campaign identity is stable and mutable configuration is append-versioned. Target membership is interval history. Campaign/day traffic, attribution, product/day facts and CPO facts are different fact families. Paid and bonus spend are never silently combined. Provider attribution remains labelled by `attribution_kind`; no account/SKU/category attribution is allocated or synthesized from campaign totals. Campaign registry, daily statistics and target membership have independent source contracts and provenance hashes. Unknown campaign families and scopes remain `UNKNOWN`.

## Temporal and operational rules

Provider network calls occur before bounded finalization transactions. Same-run recovery hash-verifies immutable payload artifacts, replays them through the same parser, validates request fingerprints and source contracts, and continues the next provider request. A new run never inherits another run's cursor. Older finalization cannot replace a newer authoritative head.

Stock/Price/Advertising capability and DQ results are optional inputs to readiness: they may limit their own capabilities but cannot move the Catalog + Sales analysis date backwards. All Factual API reads are seller-scoped, cutoff-bounded and traceable. No consumer outside `internal/factual` may read the new canonical tables directly.

## Source registry

| Contract | Method / endpoint | Pagination / request bound | Time and money semantics | Access | Acceptance |
|---|---|---|---|---|---|
| `STOCK.SKU_CURRENT` | `POST /v1/analytics/stocks` | frozen SKU batches, max 100 | observation time is collection acceptance; provider quantity/turnover field names retained | seller private OAuth; analytics rate limiter | endpoint/shape implemented; sellable and pool independence `PENDING_ACCEPTANCE` |
| `PRICE.CORE_STATE` | `POST /v5/product/info/prices` | cursor from empty, product batches up to 1000 | observed provider state; exact response currency/decimal | seller private OAuth | endpoint/shape implemented; field semantic registry `PENDING_ACCEPTANCE` |
| `PRICE.CUSTOMER_PRICE` | `POST /v1/product/prices/details` candidate | not invoked | customer-price enrichment only | seller private OAuth | `PENDING`, optional and non-blocking |
| `ADVERTISING.CAMPAIGN_UNIVERSE` | `GET /api/client/campaign` | page/pageSize, 100 | mutable config observation; provider money normalized from documented micro-unit fields where present | seller private OAuth bearer | repository contract implemented; real-seller enum acceptance `NOT_RUN` |
| `ADVERTISING.CAMPAIGN_TRAFFIC` | `GET /api/client/statistics/daily/json` | campaign batches, 10 | provider date retained as Moscow business date; exact RUB; paid/bonus/provider DRR separated when present; traffic separate from attribution | seller private OAuth bearer | source-family universality and attribution meaning `PENDING_ACCEPTANCE` |
| `ADVERTISING.SKU_TRAFFIC` | `GET /api/client/statistics/campaign/product/json` | campaign batches, 10 | direct campaign/SKU/day facts only; expense→paid; modelOrders→MODEL kind | seller private OAuth bearer | endpoint/shape implemented; live field acceptance `PENDING_ACCEPTANCE` |
| `ADVERTISING.ACCOUNT_SPEND` | `GET /api/client/statistics/expense/json` | date range | expense reconciles against campaign/day paid spend; never summed | seller private OAuth bearer | endpoint/shape implemented; live reconciliation acceptance `PENDING_ACCEPTANCE` |
| `ADVERTISING.TARGET_SCOPE` | `GET /api/client/campaign/{id}/v2/products` | page/pageSize, 100 | explicit membership observations; bid unit preserved | seller private OAuth bearer | explicit-SKU families only; other scopes remain `UNKNOWN` |
| `ADVERTISING.CAMPAIGN_ATTRIBUTION` | `GET /api/client/statistics/all_sku_promo/orders/generate` + status/download | bounded poll | async CPO order facts; promoted≠ordered SKU; unresolved orders stay unresolved | seller private OAuth bearer | async flow implemented; report row schema `PENDING_ACCEPTANCE` |

Secondary Stock sources and unverified Advertising report families remain out of canonical truth until live acceptance proves their semantics. Collectors may soft-fail optional product/expense/CPO branches into domain `PARTIAL` without inventing substitute endpoints.
