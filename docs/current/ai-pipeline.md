# AI Pipeline (current)

This document complements [`unified-analysis-publication.md`](./unified-analysis-publication.md)
(Stage 5) and the Stage 3/4 provider/analyst docs.

## Dual pipeline (Stage 5)

| Path | Role | Default |
| --- | --- | --- |
| Legacy seller AI pipeline | Existing Dashboard/Actions generation | On for sellers without Unified settings |
| Unified Analysis | Pack → provider → validate → publish | Off globally; per-seller opt-in |

Stage 5 is **ACCEPTED**. It does **not** delete the legacy pipeline or its
tables. Mass Unified rollout stays disabled
(`UNIFIED_ANALYSIS_ROLLOUT_ENABLED=false` by default). Missing
`seller_analysis_settings` row ⇒ `analysis_mode=legacy`.

## Read path after Unified visible publication

Existing Dashboard and Actions HTTP handlers continue to read
`seller_current_ai_snapshots`. On a visible Unified publish, that compatibility
pointer is updated in the same transaction as
`seller_current_unified_publications` (authoritative). Shadow publications are
never exposed on seller Dashboard/Actions routes.

## Post-sync routing

After successful post-sync recalculation, Worker may call the Unified auto-run
trigger. The trigger no-ops unless effective auto-run is true (global rollout +
global auto-run + seller auto-run + mode ∈ {shadow, unified}). Failed, partial,
and cancelled sync paths do not create Unified runs. Duplicate completion for
the same seller/metric/pack state remains idempotent (one active/successful run).

Legacy dispatch remains gated by existing `SELLER_AI_PIPELINE_ENABLED` /
dispatch logic.

## AI Chat

AI Chat continues to use the legacy chat data-pack path. Stage 5 publication
pointers do not break Chat. Smoke: `frontend/e2e/main-flow.spec.ts`
(dashboard → actions → chat → export).

## aipipeline tests

`internal/aipipeline` unit tests isolate ambient env via `TestMain` (clears
leaked `AI_PIPELINE_*` flags from the developer shell). Failures observed
during Stage 5 acceptance with contaminated env were **not** Stage 5
regressions.

## See also

- [`unified-analysis-publication.md`](./unified-analysis-publication.md)
- [`unified-analysis-provider.md`](./unified-analysis-provider.md)
- [`unified-analysis-analyst.md`](./unified-analysis-analyst.md)
