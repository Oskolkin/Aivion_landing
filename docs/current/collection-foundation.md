# Collection foundation

> Status: **DERIVED**. Normative precedence and replacement-architecture rules are defined in `docs/architecture/v1/`.

This document describes the implemented replacement foundation only. It does not switch the legacy Ozon sync runtime or implement provider collectors, canonical business facts, analysis, actions, publication, dashboard, or chat.

`collection_runs` is a seller-scoped logical collection cycle. Its explicit lifecycle is `PENDING → RUNNING → {COMPLETED, COMPLETED_WITH_LIMITATIONS, FAILED, CANCELLED}`. A terminal run cannot be restarted.

Lifecycle writes for both collection and domain runs use compare-and-swap against the seller-scoped status read by the service. A competing write therefore returns a typed stale-transition error instead of overwriting a newer state. Repeating the already-persisted target is an idempotent no-op.

`collection_domain_runs` records independent domain executions within a collection run. Domain failures and partial results are independent of the parent run. PostgreSQL validates its seller identity against the parent collection run, while repository reads are seller-scoped.

`collection_cursors` contains only provider-neutral transport checkpoints identified by `(seller_account_id, domain, cursor_key)`. It is not a business watermark, readiness signal, coverage claim, or factual truth.

`source_payloads` holds immutable provenance metadata. Response bodies are placed under the existing S3/MinIO raw bucket before their row is inserted. Every stored body has a SHA-256 hash and can be verified. If the metadata row cannot be inserted after an object write, an orphan object can remain for operator cleanup; the reverse broken-reference state is not created.

The payload domain is never caller-authoritative. Registration loads the seller-scoped Domain Run and derives the object path, fingerprint context and sanitized provenance metadata from its persisted domain before writing an object.

Request fingerprints use canonical JSON over a sanitized request identity. Metadata uses the same credential-removal layer. Authorization, cookies, API keys, OAuth/access/refresh tokens, client secrets, passwords, and generic secrets are not persisted by this foundation.

Stock, Price and Advertising use dedicated redaction profiles. Same-run recovery lists only payloads from the same seller and Domain Run, verifies object SHA-256, redaction profile, sequence, request fingerprint and endpoint-specific source-contract hash, then reuses the domain parser before continuing provider IO. New runs reconstruct their own universe/enumeration and do not inherit another run's cursor.
