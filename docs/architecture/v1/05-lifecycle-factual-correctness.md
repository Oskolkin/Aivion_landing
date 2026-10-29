# Lifecycle and factual correctness invariants

> Status: **NORMATIVE** for the Prompt 5.1 correctness gate.

- Canonical SKU identity is `(seller_account_id, ozon_sku)`. Missing SKU stays
  unresolved; product ID, offer, barcode and name are never identity fallbacks.
- Posting identity is stable, while Order linkage, provider order number,
  fulfillment scheme, product composition and split resolution are append-only
  knowledge selected at `observed_at <= cutoff`.
- FBO and FBS have independent outcomes. A permanent limitation can yield a
  non-destructive PARTIAL domain; resumable interruption cannot finalize it.
- FBS incremental collection uses v4 `last_changed_status_date` and persists
  its explicit mode and bounds in resume identity.
- Every Sales query evaluates reconciliation for its requested days. Missing,
  UNEXPLAINED_GAP and NOT_COMPARABLE cannot be READY; EXPLAINED_GAP is PARTIAL.
- Monetary facts with multiple currencies are not aggregatable without an
  explicit FX policy and therefore return an unknown/failed typed value.
- Sales maturity requires identical normalized hashes in the latest two
  distinct accepted runs. Eligible EXPLAINED_GAP is day-local and a correction
  immediately resets maturity. Complete-through is contiguous usable history.
- Current Catalog population is based on cutoff-selected versions, not stable
  identities ever observed. `UNOBSERVED` is outside the current universe.
- Category APIs distinguish seller, provider and parent-provider namespaces.
- Access state is derived only from persisted OAuth and machine-readable
  collection evidence. Current collection failure does not erase historical
  query capability.
- FactRef is context-bound, verifies deterministic result re-execution, and
  TraceFact returns persisted semantic revision provenance without raw payload,
  PII or secrets.
- ListPostings and ListReturns use bounded set-query shapes; per-row Get calls
  are prohibited.
