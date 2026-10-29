# Canonical Supply

`SUPPLY` models inbound orders and supplies, their current state, planned content, routes, timeslots, and optional acceptance evidence. Planned inbound quantities are operational plans and never mutate canonical Stock.

Collection uses seller-bound private OAuth. Order/supply identity is stable, state changes and bundle revisions are append-only, SKU links resolve through the canonical catalog, and partial bundle retrieval remains explicit.

## State and acceptance (Prompt 7.1)

State observations are **change-compressed transitions**: identical normalized provider/canonical state (and mapping status) does not append a row. **A→B→A** still produces three observations.

Acceptance is a **beta** collection path (`POST /v1/supply-order/act/summary/get` and `act/product/get`). Optional capabilities `SUPPLY.ACCEPTANCE`, `SUPPLY.ACCEPTANCE_SUMMARY`, and `SUPPLY.ACCEPTANCE_PRODUCTS` are independent of core Supply (order universe, current state, planned content, route, timeslot). Acceptance money is not Finance; acceptance does not mutate Stock or Finance.

The factual API lists inbound supplies, state history, acceptance, and per-SKU inbound facts under a cutoff-bound `FactualReadContext`. Existing contexts remain unchanged after later revisions.

Readiness retains state, bundle, acceptance, and SKU-inbound families and adds order-universe, current-state, planned-content, route, timeslot, acceptance summary/products, and history keys. Supply is optional to the Catalog + Sales analysis-date resolver.
