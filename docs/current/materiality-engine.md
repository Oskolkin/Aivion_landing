# Materiality Engine

Status: **Prompt №9.6 — `MATERIALITY_POLICY_V1_4` + `MATERIALITY_LANE_REGISTRY_V2`**

## Role

AI-free selection of which Category/SKU/Campaign Discovery may see. Account always included.

## Active policy

| Seal | Value |
|------|-------|
| Policy | `MATERIALITY_POLICY_V1_4` (exact `parseRat` / `CountFeature` / `featureExact`; V4 advertising paths) |
| Legacy | `MATERIALITY_POLICY_V1_3` / `V1_1` / `V1` frozen |
| Budget | `DISCOVERY_STANDARD_V1` (15/40/15) |
| Lane registry | `MATERIALITY_LANE_REGISTRY_V2` (35 active lanes; Traffic vs Attribution capabilities) |
| RRF k | 60 |
| Anchors | **current** exposure only (SKU sales / Campaign paid spend) |
| Numerics | Exact Decimal / `big.Rat` for money & RRF ordering |

## Immutability

- Policy/budget/lane versions: append-only (DB triggers)
- Succeeded `materiality_runs`: semantic hashes immutable; retry with same hashes reuses
- Selections: insert-once; no DELETE/replace after success
- Registry hashes must be real SHA-256 (no `000…0`)

## Ranking vs selection

- `ranking_hash` = canonical RankingArtifact (core hash + lanes + metrics + utilities + anchors) without self-hash
- `selection_hash` = ranking_hash + budget + selected refs/order
- Audit reasons persisted on `materiality_selections`; never leaked into Landscape Detail

## Production path

Sealed capabilities → Landscape `GlobalCapabilities` → `CoreFacts.Capabilities` → lane ACTIVE/INACTIVE.

Prompt №9.8 does not change Materiality policy, lane registry, weights, RRF, diversity or budgets. Ranking still uses only `IsKnown()` features. A 7–13 day seller may emit `SALES_CURRENT_EXPOSURE` when `CURRENT_7D` is KNOWN; comparison lanes (`SALES_ABSOLUTE_*`, `SALES_RELATIVE_*`, `SALES_FROM_ZERO`, `SALES_TO_ZERO`) require a KNOWN previous period. There is no hidden 14-day Materiality gate.

## Production lanes (Prompt №9.4)

All seven lane families proven on the production factual path (no MemorySource):

Sales, Stock, Advertising, Search (`SEARCH_POSITION_CHANGE`), Price (`PRICE_ABSOLUTE_CHANGE`), Supply (`SUPPLY_OVERDUE_EXPOSURE`), Campaign.

Full active lane manifest: `materiality/active_lane_manifest.go` — V4 paths (`sku.advertising.traffic.paid_spend`, `campaign.traffic.paid_spend`, `campaign.attribution.attributed_order_amount`, `campaign.attribution.calculated_drr`). SKU/Category ad-spend lanes require only `ADVERTISING.SKU_TRAFFIC`. Campaign attribution lanes require `ADVERTISING.CAMPAIGN_ATTRIBUTION` (no Traffic substitution). `DRR_ABSOLUTE_PP_CHANGE` requires BOTH Traffic and Attribution (`CapabilityMatch=ALL`) and compatible periods.

E2E: `TestPrompt96ProductionContentfulE2EV4`.

## Exact numerics (Prompt №9.2)

Ranking-relevant operational metrics use exact types (`Decimal` / `big.Rat` / integer counts). Parse failures never silently become zero. Ranking artifacts serialize canonical exact values. RRF ordering is deterministic for close values.
