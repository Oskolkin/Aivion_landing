# Prompt №9.2 Traceability Matrix

Baseline: post №8.2 + №9.1 on `codex/canonical-sales-hardening`. Next migration: **000087**.

| Part | DEFECT | LOCATION | FIX | TEST |
|------|--------|----------|-----|------|
| A | LATEST_STABLE one binding/domain | `readiness/analysisdate.go` buildContext | bind per Watermark (Domain+FactFamily) | multi-family SEARCH LATEST_STABLE |
| A | synthetic DEFAULT family | buildContext | remove DEFAULT fallback | BindingsForDomain==3 |
| B | source_contract_hash = revision_hash | `readiness/evaluator.go` wm(), persist | derive contract hash K+V+endpoint; separate mapping/revision | contract hash != mapping hash |
| B | resolve assumes hex64 = contract | `resolve/resolve.go` resolveContractHash | always derive from run fields | provenance test |
| C | provenance only in MinIO | `factual_universe_sources` schema | migration 000087 + persist all fields | DB tamper fails LoadValidated |
| C | FinalizedAt = observed/cutoff | `universe/universe.go` NewSealedContext | use source_finalized_at from run | T1≠T2≠T3 test |
| D | lagDaysForDomain uses DomainBinding | `landscape/factual_source.go` | Binding(domain, factFamily) | Search SELLER_SUMMARY window |
| E | Search prev not in Core | factual_source SKU Search block | project current+previous position | SEARCH_POSITION_CHANGE lane |
| E | Price history missing | factual_source SKU Price | LastObservedChange from history API | PRICE_ABSOLUTE_CHANGE lane |
| E | open == overdue | `materiality/from_landscape.go` category | OverdueSupplyCount from canonical OVERDUE | SUPPLY_OVERDUE lane |
| F | empty SKU campaign detail | `landscape/detail_source.go` | ListSKUCampaignPerformance factual API | truncated campaign list |
| G | error → READY_EMPTY | factual_source category/search reads | PARTIAL/UNAVAILABLE on error | injected error tests |
| H | parseFloat → 0 | `materiality/from_landscape.go` decimalFloat | exact Decimal/Rat, fail on parse | invalid decimal test |
| I | IMPLEMENTATION_NOT_PROJECTED | factual_source account/category | close safe projections | zero gap codes |
| J | fake MATERIALITY_POLICY_V1 hash | migration 085 | 000087 cleanup unused V1 row | registry real hashes |
| K | broad acceptance | prompt91 suite | per-lane assertions + prompt92 suite | all 7 lane families |
