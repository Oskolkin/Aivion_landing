# Prompt №9.1 Architecture Traceability Matrix

ARCHITECTURE_DOCS_READ = PASS  
ARCHITECTURE_CONTRACT_CONFLICT = NONE  
Baseline: f61751b (Prompt №9 committed). Latest migration: 000084.

| REQUIREMENT | CURRENT PRODUCTION | GAP | FIX | ACCEPTANCE |
|---|---|---|---|---|
| Factual API batch ads/inbound/coverage/campaign rollup | Stubs → UNAVAILABLE | A,J | Add typed batch methods + set SQL | batch + query-count tests |
| GlobalCapabilities from sealed Universe | Never set; CoreFacts.Capabilities empty | B | Map sealed assessments → CapabilityFlag | capability propagation E2E |
| Capability key alignment | Lane keys vs seed `SALES_DEMAND.ACCOUNT_DAILY` | key mismatch | Explicit map + sealed registry keys | key alignment test |
| DetailDataSource production | `Source: nil` | C | FactualAPIDetailSource + wire | Detail supporting context |
| Account/Category/SKU/Campaign Core complete | Sales-thin | D | Fill approved blocks when capability READY | Core completeness asserts |
| SKU/Campaign eligibility | List all | E,F | Eligibility policy v1 + sealed set intersect | eligibility fixtures |
| UNMAPPED CategoryDemand | Dropped when CategoryID nil | G | Index UNMAPPED bucket | reconciliation + exclusion |
| Ignored factual errors | `_, _ :=` | H | Propagate / map to block state | error≠READY_EMPTY |
| DomainWindows | Sales only | I | Per-domain source-native windows | Sales≠Search windows |
| Materiality SoT immutability | DO UPDATE + DELETE | K | Append-once + conflict | immutability tests |
| Policy/Budget/Lane immutability | DO UPDATE; zero hashes | L,M | Forward mig 085; real hashes; insert-only | registry hash + collision |
| ranking_hash semantic | Order+anchors only | N | Hash full ranking artifact + core hash | same-order different facts |
| Anchors current exposure | max(cur,prev) | O | Current-only | TO_ZERO not anchor |
| Soft diversity | Absolute 0.12 vs RRF~0.016 | P | Relative bounded fractions; POLICY_V1_1 | soft diversity tests |
| Exact numerics | float64 ParseFloat | Q | Decimal/Rat for money/exposure | close-value ordering |
| Audit persistence | Empty codes | R | ExplainSelection → DB | audit reload |
| Contract Bundle | Name-only Materiality | S | Policy+lane+budget+eligibility hashes | NO_OP sensitivity |
| Golden + production path | CoreFacts-only; empty Asynq seed | T,U | Expand golden + substantive fixture E2E | Layer A+B gates |

Eligibility (Entity Landscapes §78/§80): SKU = active OR sales OR stock>0 OR open supply OR promo OR ads; Campaign = active/planned OR spend OR attribution.
