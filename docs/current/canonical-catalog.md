# Canonical Catalog implementation

> Status: **DERIVED**. Normative precedence and replacement-architecture rules are defined in `docs/architecture/v1/`. Canonical content capability is partial.

The new Catalog path is independent from legacy `products` and `productsync`.
It uses `POST /v3/product/list` with the provider's nested `filter` object and
reads `result.items[].product_id`. A complete catalog universe is the union of
two independent streams: `filter.visibility=ALL` and
`filter.visibility=ARCHIVED`. Each stream starts every new full snapshot with
an empty `last_id`, paginates to its own terminal empty cursor, and never
reuses a cursor from another stream or a previous run.

Canonical SKU identity is `(seller_account_id, ozon_sku)`. `offer_id` and
`product_id` are mutable/provider metadata captured in `canonical_sku_versions`;
fulfilment is not SKU identity. Product list and product detail payloads are
stored through the Collection Foundation before canonical normalization. Raw
list provenance records the stream visibility and stream-local cursor; detail
provenance identifies the deduplicated union request.

Only catalog metadata is normalized: Ozon SKU, product id, offer id, name,
provider status, archive state, description category and product type. Price,
stock, commissions, promotions and finance values remain raw provenance and
are not persisted as catalog facts.

`canonical_skus` is the stable seller-scoped identity table. Metadata changes
append `canonical_sku_versions`; unchanged normalized metadata creates no new
version. `ACTIVE` and `ARCHIVED` come from the current provider response.
Both list streams complete before required detail batches are requested or
canonical/current state is written. Duplicate Ozon SKUs across the streams
collapse to one seller-scoped identity. If provider observations for that
identity disagree on current state, the deterministic result is `UNKNOWN`;
response order cannot decide between `ACTIVE` and `ARCHIVED`.

`UNOBSERVED` reconciliation runs only after both list streams and every
required detail batch succeed. Observed SKU/version writes, category
assignments, missing-SKU `UNOBSERVED` versions, complete counters and the
Catalog Domain Run compare-and-swap transition to `COMPLETED` commit in one
transaction. A list, detail, normalization, provenance or transaction failure
leaves canonical/current state unchanged and cannot reconcile absence.
`pages_received` remains the total number of successful product-list pages
across the ALL and ARCHIVED streams; operational results also expose the two
stream page/item counts, union SKU count and cross-stream duplicate count.

Taxonomy tables preserve provider category and Product Type versions. Product
Type stable identity is `(provider category, type_id)` and remains supporting
context, never seller Category identity. Only current enabled leaf provider
categories can become seller category targets. Category assignment history is
append/close based; unknown taxonomy stays `UNMAPPED`, taxonomy failure does
not invent mappings, and no historical category backfill is permitted.

The manual command creates separate Taxonomy and Catalog Domain Runs, stores
every successful provider response as an immutable raw object, and prints run
IDs, taxonomy counts/hash, catalog counters and canonical summary. If taxonomy
fails but the full catalog succeeds, the parent run is
`COMPLETED_WITH_LIMITATIONS`; SKUs are preserved with `UNMAPPED` assignments.

Migration `000062_create_canonical_catalog` adds:

- `taxonomy_sync_runs`, `provider_categories`,
  `provider_category_versions`, `provider_product_types`, and
  `provider_product_type_versions` for deterministic full-tree observations.
  Missing entries become versioned `UNOBSERVED` only after a successfully
  received and normalized full tree; failed syncs do not reconcile absence.
- `canonical_skus` for stable `(seller_account_id, ozon_sku)` identity,
  `canonical_sku_versions` for immutable metadata/state observations,
  `canonical_sku_content_snapshots` for image content, and
  `canonical_sku_relations` as an identity-preserving relation table.
- `seller_categories` and `sku_category_assignments` for seller-scoped,
  leaf-only targets and effective `observed_from`/`observed_to` history.

Database triggers reject cross-seller SKU versions, content, relations and
category assignments, reject the wrong Collection Domain, and prevent a
current disabled/unobserved/non-leaf provider node from becoming a seller
Category. Reads of canonical SKU identity require the seller key.

Run manually with `go run ./cmd/devtools collect-catalog --seller-id=1` after
the Postgres, MinIO and a valid seller Ozon connection are available.
