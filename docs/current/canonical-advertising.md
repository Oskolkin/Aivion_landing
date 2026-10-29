# Canonical Advertising

Canonical Advertising is implemented by migrations `000071` + Prompt 6.1 hardening `000072`, `internal/collection/advertising`, seller-bound Ozon Performance transport and the Advertising Factual API.

## Source-family contracts

The domain has separately versioned source contracts for campaign registry, campaign/day statistics, campaign/product daily statistics, expense reconciliation, explicit campaign-product membership and async CPO order reports.

## Target scope / identity

Campaign family from payment/type does **not** prove target scope. `PRODUCT_CPC` / `SEARCH_PROMO_CPO` start as `TargetScopeType=UNKNOWN`. Scope becomes `EXPLICIT_SKU_SET` only after a successful authoritative products enumeration (including a valid empty set). Failed target collection does not close open intervals. Product SKU identity uses verified provider field `sku` only — never arbitrary `id`.

## Attribution kinds

`orders` / `modelOrders` / `viewOrders` materialize as separate `PROVIDER_DEFAULT` / `MODEL` / `VIEW` facts. Factual `GetSKUAdvertising` never cross-sums attributed sales across kinds; summary prefers `PROVIDER_DEFAULT` and exposes `ByAttributionKind`.

## CPO resume / numeric / revisions

After successful submit, report requests are persisted (`SUBMITTED`) before polling. Same `collection_domain_run` + request fingerprint resumes the existing UUID (no second submit). New domain runs may submit new reports. CPO JSON uses Performance `UseNumber` decoding — no float64 money/ID path. Canonical CPO identity is `provider_fact_key`; unchanged rereads reuse, corrections append `revision_no`. Factual `ListCPOAttribution` returns latest revision as-known-at cutoff.

## Spend reconciliation / watermarks / capabilities

Spend reconciliations are append-only history; DQ/readiness use latest observation per business date at cutoff. Traffic `latest_complete_business_date` requires day-level usable coverage, not merely max observed date. Attribution maturity remains separate. Capabilities evaluate campaign universe/config, target scope, traffic, expense reconciliation and attribution families independently enough that CPO soft-fail does not degrade campaign traffic readiness.

Run locally:

```text
go run ./cmd/devtools collect-canonical-vertical --seller-id=1 --domain=advertising --date-from=2026-06-01 --date-to=2026-08-24
```

Real seller semantic acceptance may remain `PENDING/NOT_RUN` when credentials or a verified provider report schema are unavailable; uncertainty stays explicit in capability/DQ limitations.
