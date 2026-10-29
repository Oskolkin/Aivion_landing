# Prompt 5.1 future-knowledge audit

> Status: **DERIVED** from the implemented Factual API queries and migration 000068.

| Response field family | Knowledge source | Mutable | Cutoff rule | Result |
|---|---|---:|---|---|
| Account seller ID | `seller_accounts.id` stable identity | No | context seller | SAFE |
| SKU identity | `canonical_skus` | No | `first_observed_at <= cutoff` | SAFE |
| SKU attributes/state | latest `canonical_sku_versions` | Yes | `observed_at <= cutoff` | SAFE |
| SKU category mapping | `sku_category_assignments` interval | Yes | interval contains cutoff | SAFE |
| Seller Category identity | `seller_categories.id` | No | `first_observed_at <= cutoff` | SAFE |
| Category/provider/parent-provider attributes | latest `provider_category_versions` plus provider identity rows | Yes | `observed_at <= cutoff` | SAFE |
| Order identity | `sales_orders.ozon_order_id` | No | `first_observed_at <= cutoff` | SAFE |
| Provider Order number | latest applicable `sales_posting_versions.provider_order_number` | Yes | `observed_at <= cutoff` | SAFE |
| Posting status, scheme and Order resolution | latest `sales_posting_versions` | Yes | `observed_at <= cutoff` | SAFE |
| Posting products and SKU resolution | `sales_posting_products` owned by selected Posting version | Yes | selected version at cutoff | SAFE |
| Split parent and resolution | latest `sales_posting_relation_versions` | Yes | `observed_at <= cutoff` | SAFE |
| Return provider/link/value fields | latest `sales_return_events` | Yes | `observed_at <= cutoff` | SAFE |
| Lifecycle event fields | append-only `sales_lifecycle_events` | No after acceptance | `observed_at <= cutoff` | SAFE |
| Account/SKU demand | latest daily revision per grain | Yes | `observed_at <= cutoff`; period reconciliation rechecked | SAFE |
| Category demand | daily Sales revision plus historical assignment interval | Yes | both facts and assignment constrained by cutoff/business day | SAFE |
| Capability/DQ/watermark bindings | frozen assessment/run IDs in `FactualReadContext` | Yes | assessed/finalized at or before context cutoff | SAFE |
| Fact provenance | selected canonical revision, payload metadata and source revision hash | No after acceptance | same context cutoff; signed result re-executed | SAFE |

No Factual API response field is known to read mutable current projection state
without historical knowledge-time semantics. Stable lifecycle projection
columns remain write-side conveniences and are prohibited in cutoff reads.
