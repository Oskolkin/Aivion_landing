# Canonical Price

Canonical Price is implemented by migration `000070`, `internal/collection/price`, `POST /v5/product/info/prices`, and the Price Factual API.

Each run freezes the Catalog universe, starts with an empty provider cursor, batches product IDs, stores exact redacted responses, and atomically writes a collection snapshot plus observed price rows. Same-run recovery validates immutable artifacts and resumes the next cursor; a new run never consumes an older run's cursor.

All monetary values use exact six-decimal fixed point in Go and `NUMERIC(30,6)` in PostgreSQL. Currency is explicit and cross-currency aggregation is forbidden. Field semantics are independent: current price zero is `INVALID`; accepted optional old/minimum/reference zero values become not-set; seller promotion, retail, price index and raw marketing context remain separate. Customer price is optional premium enrichment and is not inferred from marketing fields. The current semantic registry remains `PENDING_ACCEPTANCE` until real-seller verification.

Factual reads include current, as-of, batch/list, history and price-index views. Fact references bind to the exact price observation, so historical trace verification cannot silently re-execute against the latest row.

Run locally:

```text
go run ./cmd/devtools collect-canonical-vertical --seller-id=1 --domain=price
```
