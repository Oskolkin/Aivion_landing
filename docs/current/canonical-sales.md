# Canonical Sales Demand implementation

> Status: **DERIVED**. The normative replacement architecture remains under
> `docs/architecture/v1/`. This implementation covers ordered demand only.

Canonical Sales Demand reads only `POST /v1/analytics/data` through the
seller-bound private OAuth transport. It does not read orders, postings,
returns, finance, advertising, legacy sales, `daily_*_metrics`, or the legacy
product analytics sync. Two independent physical queries belong to one
`SALES_DEMAND` Domain Run:

- SKU daily: dimensions `sku,day`; metrics `revenue,ordered_units`.
- Account daily: dimension `day`; metrics `revenue,ordered_units`.

`revenue` maps to canonical `ordered_amount`. Account data is never derived
from resolved SKUs or from response totals. The adapter preserves JSON numbers
as `json.Number`; money is normalized as exact six-scale decimal and stored as
`NUMERIC(30,6)`. Ordered units must be mathematically integral and
non-negative. Negative money, fractional units, malformed dates, unexpected
dimension/metric cardinality and duplicate business grains fail the source
contract. Provider days are persisted as the returned calendar `DATE` without
timezone shifting.

Currency is resolved by the explicit provider-market policy. The implemented
mapping is `OZON/RU → RUB`; an unknown mapping is a contract failure. This is
not a global RUB assumption and no FX conversion occurs.

Every **new authoritative run** starts each source stream at `offset=0`, uses
`limit=1000`, and requests the next page after an exact-size page. Each
successful page is redacted with
`OZON_SALES_DEMAND_REDACTION_V1`, stored in MinIO, and registered in
`source_payloads` before it can become a candidate fact. Retry remains bounded,
honours `Retry-After`, and is cancellation-aware. Endpoint pacing is owned by
the seller-bound transport and configured by
`OZON_ANALYTICS_MIN_INTERVAL` (default one minute).

Each successful page also advances a run-scoped, stream-specific Collection
checkpoint (`SKU daily` and `Account daily` are independent) with page count,
next offset and terminal marker. An interrupted `RUNNING` domain can be resumed
with `--resume-domain-run-id`: all completed pages for that exact domain run
are loaded from immutable `source_payloads` objects, hash-verified, checked
against seller/run/contract/version/redaction/page metadata, and passed through
the same decoder and normalizer as live responses. Only after the full prefix
has reconstructed candidates and duplicate-grain state does collection call
Ozon at the next offset. Terminal streams make no provider call.

A missing object, hash mismatch, wrong contract, undecodable page, duplicate or
hole fails closed as `RESUME_ARTIFACT_INTEGRITY_FAILURE`; the run is never
silently mixed with a fresh snapshot. Retryable transport interruptions leave
the domain and parent `RUNNING` with `SALES_COLLECTION_INTERRUPTED`. Repeating
resume is idempotent and does not register replayed payloads again. A new
authoritative run cannot consume another run's checkpoint and always restarts
both streams at zero. Replay is local and consumes no Ozon rate-limit budget.

Migration `000064_create_canonical_sales_demand` adds three isolated tables:

- `sales_demand_daily`: immutable seller/source-SKU/day revisions, with a
  nullable seller-scoped `canonical_sku_id` and explicit
  `RESOLVED|UNRESOLVED` state.
- `sales_account_daily`: immutable independent seller/day revisions.
- `sales_reconciliations`: one run/day comparison with separate amount and
  unit statuses (`PASS`, `EXPLAINED_GAP`, `UNEXPLAINED_GAP`,
  `NOT_COMPARABLE`) and deterministic overall precedence.

Fact hashes include values, currency and SKU resolution knowledge. Migration
`000065_harden_canonical_sales` removes global historical fact-hash uniqueness.
Deduplication compares only with the latest accepted revision: `A → A` remains
one revision, `A → B → B` remains two, and `A → B → A` correctly records three
immutable knowledge states even though the first and third hashes match.
Business-key plus `revision_no` remains unique, and seller/domain advisory
locking continues to serialize race-safe allocation. Source corrections and
later SKU resolution append a new revision while retaining old knowledge. PostgreSQL
constraints and triggers enforce revision uniqueness, provenance scope,
seller/domain ownership and cross-seller SKU isolation. Unknown source SKUs
are preserved as unresolved and never create fake Catalog identities.

`requested_from/to` records the requested interval. `coverage_from/to` is the
minimum and maximum date actually present in authoritative **Account daily**
observations; no Account rows means both values are `NULL`. Sparse SKU rows do
not define domain coverage. An observed Account row with explicit zero counts
as coverage. The outer bounds do not claim continuity: any missing internal,
leading or trailing requested day remains a `NOT_COMPARABLE` reconciliation.

Missing is never converted to zero. An explicit account zero with no SKU rows
can reconcile `PASS`; a missing account day is `NOT_COMPARABLE`, and missing
SKU/day rows are not synthesized. Reconciliation compares the independent
account value with all source SKU rows and then reports the part explained by
unresolved identities. There is no numeric tolerance or synthetic allocation.

Collection and normalization happen outside a database transaction. Final
candidate persistence, seller-scoped SKU resolution, revision allocation,
daily reconciliation, Domain Run transition and authoritative-head update
commit under one seller/domain advisory lock. One `canonical_acceptance_at` is
captured after stale-run validation and used as `observed_at` for every new fact
and reconciliation revision plus the authoritative finalization boundary in
that transaction. It is the time Aivion accepted the complete normalized and
resolved canonical knowledge, not the earlier provider receipt time.
`source_payloads.received_at` independently preserves that receipt provenance.
Consequently late `UNRESOLVED → RESOLVED` knowledge and later corrections get
new revisions with later acceptance times; old rows are never backdated or
updated.

A newer finalized run prevents an older worker from appending stale
corrections. The stale domain is `CANCELLED` with
`SUPERSEDED_BY_NEWER_AUTHORITATIVE_RUN`; the devtool cancels its parent rather
than reporting a generic infrastructure `FAILED` state. Supersession is
terminal and non-retryable. Full source retrieval with
unresolved identities, gaps, or missing account days becomes Domain Run
`PARTIAL` and parent `COMPLETED_WITH_LIMITATIONS`; trustworthy limitation-free
retrieval becomes `COMPLETED`.

Run an explicit range:

```text
go run ./cmd/devtools collect-sales --seller-id=1 --date-from=2026-08-01 --date-to=2026-08-03
```

Or the 90 last completed Europe/Moscow business days:

```text
go run ./cmd/devtools collect-sales --seller-id=1 --initial-90d
```

Resume the same still-`RUNNING` Sales domain after a retryable interruption:

```text
go run ./cmd/devtools collect-sales --seller-id=1 --resume-domain-run-id=123
```

## Future factual read invariant

Prompt 3.1 does not implement a Factual API. A future factual universe may
admit a Canonical Sales revision only when all of these hold at its cutoff:

1. the revision belongs to an authoritative accepted Sales domain run;
2. that authoritative domain run successfully finalized by the cutoff;
3. `revision.observed_at <= cutoff`;
4. the run has not been superseded by newer authoritative state in the
   applicable knowledge chronology.

The exact read SQL, readiness policy and watermarks remain Prompt 5 scope.

The command requires a seller-bound `PRIVATE_OAUTH` grant and prints only safe
run IDs, coverage, counters, reconciliation outcomes, currency, and statuses.
Real-seller live acceptance remains environment-dependent and is not simulated;
the deterministic hardening gate is covered by automated PostgreSQL and
immutable-object replay tests.
