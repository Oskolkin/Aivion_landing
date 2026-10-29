# Prompt №9 Architecture Traceability Matrix

ARCHITECTURE_DOCS_FOUND = PASS
ARCHITECTURE_DOCS_READ = PASS
ARCHITECTURE_CONTRACT_CONFLICT = NONE

Baseline: fa97a75 (Prompt 7.1+8.1 committed). Latest migration before work: 000083.
Unrelated dirty: none material (accidental empty SEAL_UNIVERSE removed).

| REQUIREMENT | SOURCE | CURRENT | GAP | IMPLEMENTATION | ACCEPTANCE |
|---|---|---|---|---|---|
| Landscape Factual-API-only | Entity Landscapes; Factual API | none | no package | landscape/** via factualapi | import/SQL gate |
| Landscape deterministic / no AI | Entity Landscapes | none | — | pure Go builders | hash + no OpenAI |
| No semantic interpretation | Entity Landscapes § | none | — | typed facts only; forbidden vocab validator | schema/static test |
| Typed entity schemas | Entity Landscapes | none | — | Account/Category/SKU/Campaign structs | typed schema test |
| Core vs Detail | Entity Landscapes | none | — | Core all eligible; Detail selected | Core/Detail tests |
| Leaf Category only | Entity Landscapes; Materiality | none | — | leaf seller-bearing filter | leaf + UNMAPPED tests |
| Global windows/capabilities/limitations | Entity Landscapes | none | — | LandscapeSet.Context/Windows/Caps/Limits | window alignment test |
| Temporal source-native windows | Temporal + Landscapes | none | — | DomainWindow with actual from/to | Sales≠Search window test |
| missing≠zero; FROM_ZERO/TO_ZERO; p.p. | Landscapes | none | — | ChangeState + Decimal comparisons | zero/pp tests |
| No unsafe non-additive agg | Landscapes | none | — | no unique_users SUM; no stock+inbound | non-additive tests |
| Bounded supporting context | Landscapes Detail | none | — | TruncatedList metadata | truncation test |
| No Landscape persistent SoT | Physical Plan | none | — | artifacts only | no sku_landscapes tables |
| Materiality AI-free typed lanes | Materiality Engine | none | — | lane registry + typed funcs | lane registry test |
| Exposure guards; FROM/TO_ZERO | Materiality | none | — | relative floor + transition lanes | golden fixtures |
| Anchors + RRF + soft diversity | Materiality | none | — | phase A ranking | RRF/diversity tests |
| Budget ≠ Ranking | Materiality | none | — | ranking_hash vs selection_hash | budget separation |
| Audit not in Detail | Materiality | none | — | separate ranking artifact | leak scan |
| Continuity no-base only | Prompt9 + Action/Publication | stage stub | — | CONTINUITY_INITIAL_NO_BASE_V1 | continuity tests |
| Stages 3–6 wired; 7 not run | Pipeline Orchestration | 1–2 only | — | ProductionHandlers 1–6; frontier | Asynq E2E |
| PG SoT; MinIO artifacts; Asynq | Physical Plan | Prompt8.1 | extend | materiality_* tables + artifacts | PG+Redis E2E |

Tuning params (max_skus, rrf_k, floors, weights): PILOT_DEFAULT versioned in MATERIALITY_POLICY_V1 / DISCOVERY_STANDARD_V1 — not architecture conflicts.
