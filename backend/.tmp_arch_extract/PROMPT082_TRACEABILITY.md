# Prompt №8.2 Traceability Matrix

ARCHITECTURE_DOCS_READ = PASS (v1 extract + prior Prompt 8/9 docs)  
ARCHITECTURE_CONTRACT_CONFLICT = NONE  
Baseline HEAD: f61751b + uncommitted Prompt №9.1 working tree. Latest migration: 000085 → next **000086**.

| # | REQUIREMENT | CURRENT | DEFECT | FIX | ACCEPTANCE |
|---|---|---|---|---|---|
| 1 | Domain+FactFamily binding identity | `map[domain]DomainBinding` | overwrite | typed FactFamilyKey map; Binding(domain,family) | multi-family seal test |
| 2 | Historical watermarks AS KNOWN AT | mutable PK upsert | erases past | `domain_watermark_revisions` append-only + AS-KNOWN-AT query | A→B→A + cutoff race |
| 3 | Real source contract provenance | Domain/`v1` synthetic | fake | load from collection_domain_runs / source metadata | contract key/version equality |
| 4 | Explicit family↔capability map | HasPrefix heuristic | fuzzy | versioned registry; no prefix | stock multi-cap fixture |
| 5 | DQ fail-closed | DB err → nil/NONE | fail-open | propagate errors; NOT_EVALUATED only on ErrNoRows | injected error tests |
| 6 | Entity sets via Factual API | ComputeEntitySetMembers SQL | boundary | factual ListSKUs/Campaigns/Category mappings | no canonical SQL in analysis |
| 7 | Semantic entity members | category uses DB IDs | unstable hash | ozon_sku + category semantic key | DB-ID independence |
| 8 | SEALED ≡ Resolved bindings | FinalizedAt=Cutoff; WMHash=SourceHash | synthetic | real finalize + separate hashes | resolved/sealed equality |
| 9 | HMAC secret externalized | hard-coded literal | security | FACTUAL_TOKEN_HMAC_SECRET | no literal; config fail |
| 10 | Analysis canonical SQL gate | allowlist only | weak | denylist gate on analysis/** | fixture detection |

Prompt №9.1 regression mandatory after changes.
