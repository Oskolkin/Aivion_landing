# Canonical Sales Lifecycle implementation

> Status: **DERIVED**. Normative precedence remains in `docs/architecture/v1/`.

Canonical Sales Lifecycle is fulfillment evidence, not Sales Demand. It never
writes or derives `ordered_amount`/`ordered_units`, never aggregates posting
quantities into demand, and never uses finance amounts. Demand remains owned
exclusively by `POST /v1/analytics/data`.

## Sources and authentication

The polling collectors use the seller-bound Private OAuth Bearer transport:

- FBO list: `POST /v3/posting/fbo/list`;
- FBO detail: `POST /v2/posting/fbo/get`;
- FBS/rFBS-family list: `POST /v4/posting/fbs/list`;
- FBS detail: `POST /v3/posting/fbs/get`;
- FBO/FBS returns: `POST /v1/returns/list`.

The deprecated FBS v3 list is prohibited in canonical code. There is no
Client-ID/API-Key fallback, conditional-cancellation is not treated as a
general cancellation source, financial data and barcodes are not requested,
and no seller-mutating Ozon action is called.

Each adapter has a checked-in versioned contract and deterministic semantic
hash. List pages and required details are structurally redacted before MinIO,
then registered in `source_payloads`. Separate profiles remove buyer/customer,
addressee, phone, email, address, legal/tracking data, barcodes, IMEI, digital
codes and auth material while retaining required product facts.

## Entity and history model

`sales_orders` is stable identity `(seller, Ozon order_id)`. It contains no
status, products or fulfillment state. One Order can own any number of
`sales_postings`; no Order Items table exists. A conflicting non-empty order
number is `ORDER_IDENTITY_CONFLICT`, and posting number is never an Order
surrogate.

`sales_postings` is stable identity `(seller, posting_number)`. Scheme is one
of `FBO`, `FBS`, `RFBS`, `UNKNOWN`; FBS-family scheme comes from explicit
provider schema. `UNKNOWN → known` is enrichment, while incompatible known
schemes are `POSTING_SCHEME_CONFLICT`.

Mutable state is append-only in `sales_posting_versions`. The state hash covers
status/substatus, scheme, provider lifecycle and cancellation fields,
warehouse/provider context, and a canonically sorted complete product
composition. Dedup compares only the latest accepted version, so A→B→A is
three versions. `source_received_at` is provider receipt; `observed_at` is the
single canonical acceptance boundary used by that finalization.

`sales_posting_products` belongs to a specific version, never to an Order.
Quantity is non-negative integral, money is exact six-scale decimal, and SKU
resolution is seller-scoped. Unknown SKUs remain explicit `UNRESOLVED`; no
Catalog identity is manufactured. Posting price remains commercial lifecycle
evidence and never becomes Sales Demand or Finance.

Explicit FBS `parent_posting_number` creates a `SPLIT` relation. Naming
patterns and quantity changes do not. An unseen parent is retained by number
as `UNRESOLVED` and resolves when that stable posting appears. Self-links and
detected cycles fail as `POSTING_LINEAGE_CYCLE`.

`sales_returns` is stable `(seller, returns[].id)` identity. Append-only
`sales_return_events` preserves provider type/schema/reason, source references,
quantity/price, visual/logistic timestamps and compensation state. Mutable
observations append versions with latest-only dedup and allow A→B→A. `source_id`
does not merge Return identities. Order, Posting and SKU links resolve
independently; missing links never discard source Return truth.

A provider Return is deliberately not classified as CUSTOMER_RETURN,
refusal-at-delivery, technical return or seller/buyer fault. Provider fields
are retained for a later versioned policy.

## Normalized temporal events

The closed v1 taxonomy is `POSTING_CREATED`, `POSTING_SPLIT`,
`POSTING_DELIVERING`, `POSTING_DELIVERED`, `POSTING_CANCELLED_FULL`,
`POSTING_CANCELLED_PARTIAL`, and `RETURN_CREATED`. Time quality is explicit:

- `PROVIDER_EXACT`: documented event timestamp; `effective_at` is populated;
- `PROVIDER_DATE`: documented event date only;
- `OBSERVED_APPROX`: transition occurred between two observations;
- `UNKNOWN`: event is known but cannot be localized.

`observed_at` is never substituted for `effective_at`. Polling transitions
have a bounded observation interval and null `effective_at`; business date is
set only when the entire interval lies in one Moscow business day. A terminal
posting first seen historically gets an undated `UNKNOWN` status event, not an
invented delivery/cancellation date. `logistic.return_date` remains a provider
field and is not treated as Return creation time.

A quantity decrease, split or provider correction never creates partial
cancellation by itself. `POSTING_CANCELLED_PARTIAL` is reserved for explicit
supported cancellation evidence; the current polling mapping emits none.
Deterministic dedupe keys include concrete version/relation identities, so
replay is idempotent without blocking later repeated A→B transitions.

## Collection, resume and authority

POSTINGS and RETURNS are independent Collection domains. POSTINGS has separate
FBO and FBS cursors/results. Returns uses `limit=500`, `last_id`, and
`has_next`; posting streams reject empty, repeated or non-advancing cursors.
Every new authoritative run starts from the initial cursor.

Same-run resume verifies the run-scoped checkpoint plus immutable list/detail
artifacts: seller/run, contract/version, request fingerprint, redaction
profile, SHA-256, sequence and cursor continuity. Live and replay use the same
decoder/normalizer. Completed detail artifacts are reused. A missing/corrupt
object, wrong contract or page hole fails closed as
`RESUME_ARTIFACT_INTEGRITY_FAILURE`.

Network calls happen outside finalization. Canonical identities, versions,
products, relations, events, technical counters, Domain transition and
authoritative head commit in one seller/domain advisory-locked transaction.
A newer POSTINGS or RETURNS head cancels a stale worker without canonical
writes. Unresolved SKU/link/lineage or unknown scheme produces `PARTIAL`, not
false completeness. Coverage outer bounds come only from observed provider
temporal fields; requested bounds are not copied as coverage.

Run a bounded explicit collection:

```text
go run ./cmd/devtools collect-lifecycle --seller-id=1 --date-from=2026-08-01 --date-to=2026-08-03 --streams=all
```

Or initial 90 completed Moscow days, and same-domain resume:

```text
go run ./cmd/devtools collect-lifecycle --seller-id=1 --initial-90d --streams=all
go run ./cmd/devtools collect-lifecycle --seller-id=1 --streams=postings --resume-domain-run-id=123
```

Push ingestion is not implemented; polling is authoritative and sufficient.
Real non-empty seller acceptance remains pending for provider status/schema,
exact timestamps, split/partial-cancellation evidence and Returns filter/link
semantics. Prompt 5 owns Factual API reads, capabilities/DQ, readiness,
watermarks, factual-universe cutoff logic and any safe daily/cohort metrics.

## Prompt 5.1 correctness gate

Migration `000068` makes fulfillment scheme and Order linkage append-only
Posting-version facts and adds append-only split-relation revisions. Older
cutoffs therefore cannot observe later scheme, Order or parent resolution.
Missing source SKU remains null/`UNRESOLVED`; product ID, offer, barcode and
name are never identity fallbacks. Currency fallback is available only through
the versioned OZON/RU provider-market policy.

FBO and FBS finalize independently. Permanent unavailability/failure in one
stream permits completed facts from the other to materialize as `PARTIAL`;
transient interruption remains resumable and never finalizes an unsafe partial
universe. Zero-row success is `EMPTY_VALID`. FBS v4 supports explicit
`INITIAL_RANGE`, `EXPLICIT_RANGE`, and `INCREMENTAL_CHANGED_STATUS`; the latter
uses and checkpoints `last_changed_status_date`. Persisted Detail artifacts are
reused, and `NeedPostingDetail` requests Detail only for missing required
Order/product lifecycle fields.
