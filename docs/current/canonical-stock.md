# Canonical Stock

Canonical Stock is implemented by migrations `000069` + Prompt 6.1 hardening `000072`, `internal/collection/stock`, the seller-bound Ozon adapter and the Stock Factual API.

## Count semantics

- `expected_sku_count` / `requested_sku_count`: frozen Catalog universe size for the run
- `received_sku_count`: distinct provider SKU observations only (never includes synthetic absences)
- `resolved_sku_count` / `unresolved_sku_count`: resolution of provider-returned rows only (`resolved + unresolved <= received`)
- `missing_response_sku_count`: requested SKUs absent from a completed successful batch response
- `batch_unavailable_sku_count`: requested SKUs whose batch failed/interrupted (`SOURCE_BATCH_UNAVAILABLE`)
- `position_count`: provider rows + synthetic absence placeholders

## Missing-response correctness

Requested SKUs absent from a complete batch response are persisted as absence lines with:

- quantity UNKNOWN / reason `MISSING_PROVIDER_RESPONSE_ROW`
- `source_payload_id` NULL (no fake payload FK)
- provenance via snapshot + domain run + requested universe + completed batch payloads

Failed batches are **not** classified as missing-response; they use `SOURCE_BATCH_UNAVAILABLE` and snapshot `PARTIAL`.

## Watermarks / pools

- `latest_observed_at` may advance on PARTIAL snapshots
- `latest_complete_snapshot_at` advances only for `completeness_status=COMPLETE`
- Unverified pools are never summed into sellable totals (`STOCK_POOL_DOUBLE_COUNT_SAFETY=PASS`)
- Physical pool independence remains `STOCK_POOL_SEMANTIC_VALIDATION=PENDING_ACCEPTANCE` until live-seller proof

Factual reads include current/batch/list stock, location breakdown, snapshots, history and exact 7/30-day coverage. Absence reasons surface as UNKNOWN quantity limitations.

Run locally:

```text
go run ./cmd/devtools collect-canonical-vertical --seller-id=1 --domain=stock
```
