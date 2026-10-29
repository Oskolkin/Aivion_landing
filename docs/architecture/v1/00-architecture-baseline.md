# Architecture baseline v1

Status: **NORMATIVE**
Scope of this stabilization: Ozon transport → Collection → Canonical Catalog. Sales and all later layers are intentionally excluded.

## Replacement architecture

The Unified Canonical Data Model (UCDM) is the single target factual architecture. The target runtime is:

`Ozon → Collection/provider adapters → Canonical UCDM → Capabilities/DQ → Internal Factual API → Factual Universe → Landscapes → Materiality → Discovery → Evidence Expansion → Deep → Action Reconciliation → Publication → Dashboard/Actions/Chat`.

Legacy runtime may remain until production cutover, but it is not a foundation for new canonical semantics. Dual-read and dual-write are not target architecture; canonical collectors must not fall back to legacy facts, `daily_*` truth, legacy credentials, or the old AI pipeline. The future Internal Factual API is the only read boundary above UCDM. Dashboard, Chat, Discovery and Evidence must not read canonical SQL directly.

Backend owns identity, normalization, temporal semantics, canonical facts, data quality, capability and provenance. Missing is not zero. Provider identity is never guessed: ambiguity becomes an explicit conflict, limitation or DQ issue.

## Stable domain rules

- Stable Action Target allowlist: `ACCOUNT`, `CATEGORY`, `SKU`, `CAMPAIGN`.
- Sales Demand owner: `/v1/analytics/data`.
- Finance is not Sales. Advertising attribution is not Canonical Sales.
- Stock is a snapshot domain. Price is a snapshot/versioned domain.
- Funnel must not manufacture mathematically false conversion.
- Factual Universe uses an **AS KNOWN AT** cutoff.
- Discovery and Deep are stateless AI calls.
- Evidence is deterministic expansion from a closed catalog.
- Publication is immutable; projection is deterministic.
- Chat does not create persistent Actions.

## Category ownership

Provider Category is not a seller-bearing Category Action Target. Provider taxonomy is a global provider-market reference (`OZON/RU` today), with provider category/product-type identities independent of sellers. A Seller Category is seller-scoped and exists only when a usable leaf provider category is present in that seller's universe. Downstream contracts must never interchange provider-category IDs and seller-category IDs. Product Type remains taxonomy metadata and is never an Action Target.

Taxonomy execution may currently be seller-triggered, but global writes are serialized by provider+market and guarded by a monotonic authoritative head. Seller-specific state starts at `seller_categories` and SKU assignments.

## Ozon authentication boundary

The production target is Private Application OAuth `authorization_code` with encrypted seller-bound access and refresh tokens. The same seller-bound Bearer access token authorizes Seller API and Performance API. Canonical collectors request “authenticated call for seller X”; they do not receive Client-Id, API-Key, refresh token, client credentials or token-exchange details. There is no canonical OAuth-to-legacy fallback. Legacy credential transport remains compatibility-only for the legacy runtime.

Refresh is coordinated under a database row lock. Disconnect clears/revokes the application connection state; it does not delete the seller root or physically purge canonical data. Purge is a separate protected administrative lifecycle.

## Collection and Catalog invariants

- Complete authoritative Catalog enumeration is the union of `ALL` and `ARCHIVED`, each starting with an empty cursor.
- Canonical write, missing-to-`UNOBSERVED` reconciliation, counters, run completion and authoritative-head advance are one transaction.
- A seller+domain advisory lock plus monotonic run head prevents an older run from overwriting a newer finalized universe. Retrying the same finalize is idempotent.
- Canonical SKU identity is `(seller_account_id, ozon_sku)`. Offer ID and provider Product ID are versioned metadata.
- Simultaneous different Product IDs for one SKU in one authoritative run are `SKU_IDENTITY_CONFLICT`; no candidate is chosen and no finalization occurs.
- Nested `detail.sources[].sku` is retained only as raw provenance until Ozon semantics are explicitly accepted; it is not identity input.
- Category resolution and Product Type resolution are independent typed states: `RESOLVED`, `UNRESOLVED`, `NOT_APPLICABLE`, `CONFLICT`.
- Canonical content support is **PARTIAL** (`CATALOG_CONTENT_PARTIAL`); no full content-quality/SEO capability is claimed.

## Reproducibility and source safety

Each canonical adapter exposes an immutable source contract key/version, endpoint family and deterministic semantic mapping version/hash. Hashes change only when source-to-canonical semantics change—never from time, build or Git metadata. Domain runs persist this identity.

Every replay payload passes a versioned, domain-aware structural redactor before MinIO. Provenance persists the redaction profile. Authorization headers, OAuth/access/refresh tokens, API keys, client secrets and cookies are forbidden in artifacts, metadata, logs, fixtures, frontend DTOs and API responses.

## Physical authority

“Physical UCDM” was an earlier physical proposal. Its ideas may guide engineering, but if it differs from the final Physical Implementation Plan, the final plan has precedence. This baseline establishes logical boundaries; the current final Physical Implementation Plan is the physical implementation authority.
