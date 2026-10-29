# Dashboard header / health / actions — baseline

Date: 2026-07-20  
Seller 1 current before this task:

| Field | Value |
| ----- | ----- |
| publication_id | **53** |
| run_id | **81** |
| metric_date | 2026-07-08 |
| dashboard_content_status | ready |
| prompt_version (run) | unified_analyst_v1_17 |
| schema_version | aivion_unified_analysis_v5 |
| health_score | **0** |
| severity | critical / «Критическое состояние» |
| actions (API) | **2** |
| top_actions / quick_actions | 2 |
| data_is_stale | true |
| source_freshness_lag_days | 12 |
| data_quality_notes | `Данные не обновлялись 12 дн.` |

## Root causes

### 1. Stale banner
`DateFreshnessNote` injects note text → FE `buildDashboardQualityBanners` classifies as stale → title «Источник данных давно не обновлялся». Metadata fields remain unused for UI. Fix: stop emitting the stale banner in FE; keep API freshness fields.

### 2. Period caption
Single long `period_context.caption` built in `health_period_actions.go`. FE only displays that string. Fix: add structured caption lines (`caption_primary` / `caption_secondary`) from BE dates; FE renders two lines.

### 3. AI-resume empty space
`max-w-prose` on text column inside full-width card (`seller-pulse-section.tsx`). Fix: widen to ~max-w-5xl / full readable width; optional 2-col key points on desktop.

### 4. Health score 0
Narrative health block used non-pointer `int32`; missing JSON → 0; adapter/API then expose `0`. FE maps 0 → «Критическое состояние». Fix: pointer semantics + reject silent zero-default; AI must emit real score with factors.

### 5. Only 2 Actions
Prompt/validators allowed `material_actions_found` with ≥1 Action. No `execution_tier`. Soft prompt discouraged filling quota. Fix: schema v6 + prompt v1_18: 5–15 Actions, ≥3 `required`, validators gate publish.

## After fix (2026-07-20)

| Field | Value |
| ----- | ----- |
| publication_id | **59** |
| run_id | **89** |
| prompt_version | unified_analyst_v1_18 |
| schema_version | aivion_unified_analysis_v6 |
| health_score | **71** |
| health_status | attention / «Требует внимания» |
| actions | **5** (3 required + 2 recommended) |
| top/quick actions | 3 required (IDs match) |
| period | two-line caption_primary / caption_secondary |
| stale banner (UI) | **absent** (metadata `data_is_stale=true` retained) |

Failed attempt before success: run **88** (`structured_output_missing`, empty SO text); not published.
