# Canonical Search

`SEARCH` stores seller SKU search summaries, deterministic top-N query details, and optional Premium market-search evidence. SKU summary totals own provider GMV and order-count semantics; detail rows are not a replacement total.

## Grains (Prompt 7.1)

| Family | Grain | Table |
|---|---|---|
| Seller Summary | **SELLER × SKU × PERIOD × REVISION** | `search_sku_summaries` |
| Query Details | **SKU × Query × PERIOD** under `TOP_N_PROVIDER_RANKED` retrieval | `search_query_details` |

Legacy `search_seller_summaries` is **wrong-grain** (query-primary). Do not write it. Readiness and the Factual API use `search_sku_summaries` only; wrong-grain rows alone can never make `SEARCH.SELLER_SUMMARY` READY.

## Collection

Current and previous seven-day periods are collected through a requested through-day. Query identity uses conservative Unicode case folding, trimming, and whitespace collapse. Unique users are non-additive.

- **product-queries** (`SEARCH.SELLER_SUMMARY`): request body requires non-empty `skus[]`; exhaust all `has_next` pages per SKU batch.
- **`limit_by_sku`** (default ≤15): max **query rows per SKU** on details — not a cap on how many SKUs are requested.
- **`DetailSKUSelectionLimit`**: separate cost/rate policy for how many SKUs receive details; independent of `limit_by_sku`.

`search_queries.display_query` is the immutable first-observed display string. Later display variants append `search_query_versions` for AS-KNOWN-AT safety.

Summary and detail use latest-only semantic dedupe with **A→B→A** revision cycles (uniqueness is grain + `revision_no`).

## Factual and readiness

The factual API exposes `GetSKUSearchSummary` / `BatchGetSKUSearchSummary` / `ListSKUSearchSummary` / `GetSearchSummarySeries`, plus query details, category, and market reads under a cutoff-bound `FactualReadContext`.

`SEARCH.SELLER_SUMMARY` is assessed independently of `SEARCH.QUERY_DETAILS` and Premium market search: optional detail/Premium failure cannot invalidate a successful seller summary. Readiness also exposes granular user, position, conversion, GMV, order-count, market, and history keys.
