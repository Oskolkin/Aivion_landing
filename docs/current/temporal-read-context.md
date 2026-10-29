# Temporal read context

`temporal_policy_v1` uses Europe/Moscow business dates. Standard inclusive periods are Analysis Day, Previous Day, Current 7D, Previous 7D and Trend 30D. Explicit periods are validated and never silently clamped beyond the frozen analysis date.

`LATEST_STABLE` context creation captures `Clock.Now()` exactly once. The cutoff, authoritative runs, capability assessment IDs and policy versions form a deterministic context hash. Subsequent reads do not re-resolve “latest” and do not hold a long database transaction.

`SEALED_UNIVERSE` (Prompt №8.1) never captures a new cutoff: it restores bindings/watermarks/capabilities/DQ from the sealed Universe only. Optional operational provenance IDs may be absent; semantic binding/hash/state remain mandatory.

## Fact-family bindings (Prompt №8.2)

Domain bindings are fact-family aware. Multiple families under one domain (e.g. `ADVERTISING.TRAFFIC` + `ADVERTISING.ATTRIBUTION`) each keep their own watermark, contract, and capability linkage. ContextHash incorporates the full fact-family set — overwriting by domain name alone is forbidden.

SEAL_UNIVERSE entity-set enumeration uses a ModeLatestStable read context built from the accepted RESOLVED_CONTEXT bindings and the run cutoff (not a fresh “latest” resolve).

## Watermark revisions

Live `domain_watermarks` is a mutable projection. Historical readiness for Analysis uses append-only watermark revisions queried AS KNOWN AT `analysis_cutoff_at`. Late ingestion after the cutoff cannot rewrite the frozen Universe source binding.

Revisionable facts select the latest accepted version at or before cutoff with deterministic revision/id tie-breaking. Entity existence is also cutoff-bound. Event business-period queries use `business_date`; `observed_at` is never substituted for an unknown effective/business date. Late ingestion therefore cannot change results or later keyset pages under the same sealed context.

## LATEST_STABLE producers (Prompt №9.2)

Every non-sealed context builder (`readiness.buildContext`, devtools helpers, tests) emits **one `DomainBinding` per `(Domain, FactFamily)` watermark** — never one binding per domain. Multi-family domains (`SEARCH.SELLER_SUMMARY`, `SEARCH.QUERY_DETAILS`, `SEARCH.MARKET_SEARCH`; `ADVERTISING.TRAFFIC`, `ADVERTISING.ATTRIBUTION`; `SUPPLY.STATE`, `SUPPLY.BUNDLE`, `SUPPLY.ACCEPTANCE`) must all appear in `BindingsForDomain`. `DomainBinding(domain)` remains valid only when exactly one family exists. Synthetic `FactFamily = "DEFAULT"` is forbidden when multiple families are registered.

SEALED and LATEST_STABLE bindings for the same factual state share compatible fact-family identity semantics.
