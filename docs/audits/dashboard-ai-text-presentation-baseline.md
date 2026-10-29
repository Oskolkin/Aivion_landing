# Dashboard AI text presentation — baseline

Date: 2026-07-20  
Seller baseline publication before this change: **pub 47 / run 72** (seller 1, metric_date 2026-07-08).

## Data path

```text
Analysis Pack
→ Unified Analyst prompt (unified_analyst_v1_16)
→ structured output (aivion_unified_analysis_v5)
→ outputvalidator (+ diagnostics_contract)
→ publication adapter (ProjectDashboard)
→ frozen snapshot + NarrativePayload
→ Dashboard API GET /dashboard/home
→ frontend SellerPulseSection / AccountHealthCard
```

## Seller Pulse — current contract

| Layer | Fields |
| ----- | ------ |
| AI output | `seller_pulse.narrative.{status,tone,headline,summary,data_quality_note,important_facts[]}` |
| important_facts item | `title`, `explanation`, `fact_ids`, `entity_ids`, `severity`, `primary_section=seller_pulse_important` |
| Dashboard API | `seller_pulse.narrative.{headline,summary,important_notes[],quick_actions[]}` |
| important_notes | `title`, `text`, `tone` (fact_ids **not** currently projected) |

### Where text is formed

- AI generates headline + summary + important_facts (prompt requires summary as **3–5 long sentences**).
- Adapter maps important_facts → important_notes (title/explanation → title/text).
- Frontend `PulseSummaryStrip` collapses to **one paragraph**: prefers `summary`, else `headline`. Does **not** show headline and summary as separate levels. Does **not** list key points inside the AI card.
- `important_notes` render in sibling card «Что важно знать» (not inside AI-резюме).
- `finalizeSellerFacingDashboardResponse` sets `DisplayHeadline=false` (module title never uses AI headline).
- Fallbacks exist in `seller-pulse-view.ts` when narrative missing.
- No sentence-splitting of AI blob for structure; `formatNarrativeSentence` only ensures trailing period.
- No `line-clamp` on pulse summary (confirmed in tests).

### Root cause of dense pulse presentation

Structured AI fields already exist, but the presentation layer **flattens** headline/summary into a single prose block and parks key points in a separate knowledge card — so users see one long paragraph without hierarchy.

## «Что улучшит индекс» — current contract

| Layer | Fields |
| ----- | ------ |
| AI output | `account_summary.improvement_summary`, `improvement_levers[]` |
| lever | `title`, `explanation`, `factor_id`, `linked_action_id`, `expected_direction` |
| Dashboard API | same under `ai_diagnostics.account_summary` |

### Where text is formed

- AI generates levers + improvement_summary.
- Frontend `buildHealthImprovementSummary` **joins all levers into one space-separated paragraph** (`title: explanation` × N).
- `linked_action_id` is projected by adapter but **never used** in AccountHealthCard UI.
- Static section title «Что улучшит индекс» already exists; no second system title.
- Empty state mentions «ИИ-сводки».

### Root cause of dense improvement presentation

Structured levers exist, but FE concatenates them into a single dense text blob and drops Action links.

## Terminology baseline

Prompt currently requires seller-facing **«ИИ» not «AI»**.  
Frontend chrome uses many `ИИ-*` labels (чат, диагностика, вывод, сводка).  
Some fallbacks already say `AI-вывод` (inconsistent).

## Schema capability decision

Current **v5 fields are sufficient** to express the target hierarchy (headline / summary / important_facts ≈ key_points / improvement_summary ≈ intro / improvement_levers ≈ items).  

**No schema version bump required** for structural fields. Changes: prompt role tightening, validators, adapter fact_id passthrough, frontend structured rendering, terminology `ИИ`→`AI`.

Prompt version will bump to **`unified_analyst_v1_17`** (instruction semantics change).
