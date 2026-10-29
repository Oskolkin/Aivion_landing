# Aivion architecture v1 — source of truth

This directory is the repository entry point for the replacement architecture. Read [00-architecture-baseline.md](00-architecture-baseline.md) before using any older design, stage, audit, progress, dashboard, unified-analysis, or AI-pipeline document as an implementation contract.

## Document precedence

1. **NORMATIVE** — documents in this directory, plus a later explicitly named final Physical Implementation Plan. They define target boundaries and invariants.
2. **DERIVED** — domain implementation notes under `docs/current/` that explicitly conform to this baseline. They may explain current code but cannot override a normative rule.
3. **HISTORICAL / SUPERSEDED** — all stage plans, progress reports, audit snapshots, old unified-analysis/AI-pipeline/dashboard semantics and intermediate physical proposals unless promoted by a normative document. They describe history or the still-running legacy product only.

When documents conflict, the higher class wins. Within one class, the explicitly final and later document wins. Code and migrations describe current physical state, but do not silently redefine the target architecture.

## Supersession registry

| Existing document family | Status for replacement architecture | Permitted use |
|---|---|---|
| `docs/current/collection-foundation.md`, `docs/current/canonical-catalog.md`, `docs/current/canonical-sales.md` | DERIVED | Current Collection/Catalog/Sales implementation detail subject to this baseline |
| `docs/current/unified-analysis-*`, `docs/current/ai-pipeline.md` | HISTORICAL / SUPERSEDED | Legacy runtime operation and migration evidence only |
| `docs/stage-*`, `docs/actions-ai-first-*`, dashboard design/validation docs | HISTORICAL / SUPERSEDED | Product history and legacy compatibility only |
| audits and progress reports | HISTORICAL / SUPERSEDED | Evidence at their stated date, never a target contract |
| earlier “Physical UCDM” / intermediate physical proposals | HISTORICAL / SUPERSEDED | Engineering guidance only |
| final Physical Implementation Plan | NORMATIVE physical authority | Final physical schema and implementation authority |

Documents do not become normative merely because their path contains `current`.

## Implementation status

- [Canonical Collection Foundation](../../current/collection-foundation.md)
- [Canonical Catalog](../../current/canonical-catalog.md)
- [Canonical Sales Demand](../../current/canonical-sales.md)
- [Canonical Sales Lifecycle](../../current/canonical-sales-lifecycle.md)
- [Canonical Factual API](../../current/factual-api.md)
- [Business capabilities](../../current/business-capabilities.md)
- [Data quality and readiness](../../current/data-quality-readiness.md)
- [Temporal read context](../../current/temporal-read-context.md)
- [Lifecycle and factual correctness invariants](05-lifecycle-factual-correctness.md)
- [Stock, Price and Advertising boundary](06-stock-price-advertising.md)
- [Canonical Stock](../../current/canonical-stock.md)
- [Canonical Price](../../current/canonical-price.md)
- [Canonical Advertising](../../current/canonical-advertising.md)
- [Canonical Funnel](../../current/canonical-funnel.md)
- [Canonical Finance](../../current/canonical-finance.md)
- [Canonical Search](../../current/canonical-search.md)
- [Canonical Promotions](../../current/canonical-promotions.md)
- [Canonical Account Health](../../current/canonical-account-health.md)
- [Canonical Supply](../../current/canonical-supply.md)

Prompt №10 Discovery Runtime is implemented (stages 7–9). Prompt №10.1 Discovery grounding & runtime correctness hardening is closed. Prompt №11 Evidence Expansion is **CLOSED**. Prompt №12 Deep Analysis is **CLOSED**. Prompt №13 production stages **1–19 implemented and CLOSED** (Prompt №13.3 acceptance). Prompt №13.1 hardened CONTINUITY_REQUIRED Evidence + assessment persistence — CLOSED. Frontend/Dashboard read migration and Chat v2 are **NOT IMPLEMENTED**.
