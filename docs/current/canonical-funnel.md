# Canonical Funnel

`FUNNEL` stores provider funnel observations without importing Sales revenue or ordered-unit truth. Daily additive metrics and provider-computed direct-period metrics are separate fact families.

Collection uses Ozon Analytics Data through seller-bound private OAuth. Account and SKU grains are retained; SKU identities resolve to the canonical catalog when possible. Missing observations are unknown, not zero, and direct conversion/session values are never reconstructed by summing daily rows.

## Revisions (Prompt 7.1)

`state_hash` is semantic only: value, unit, temporal/aggregation semantics, surface, provider metric name, registry/contract hashes, canonical SKU id (or `NULL`), and resolution status. It excludes pointer addresses, `revision_no`, `observed_at`, and DB row ids — no revision chaining through prior hashes.

Latest-only dedupe against the current head:

- **A→A** — same semantic hash as latest revision: skip write
- **A→B** — different hash: append next `revision_no`
- **A→B→A** — return to a prior semantic state: append a new revision (uniqueness is grain + `revision_no`, not `state_hash`)

The factual API reads account funnel periods under a cutoff-bound `FactualReadContext`. Revisions are append-only and selected AS-KNOWN-AT the context cutoff.

Readiness exposes the family keys plus granular Search/PDP impressions, cart adds, view sessions, conversions, account, SKU, and history capabilities. Funnel remains optional to the Catalog + Sales analysis-date resolver.
