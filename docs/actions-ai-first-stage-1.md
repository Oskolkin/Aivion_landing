# Actions AI-first Stage 1

## Why this exists

Stage 1 prepares a safe contract and taxonomy layer for future AI-first Actions generation from Seller AI Data Pack.

Stage 1 does not change active runtime. Existing candidate-first Actions generation remains the source of persisted action_items until Stage 2.

## Candidate-first vs AI-first

- Candidate-first (current): backend builds candidates first, AI formats candidates into actions.
- AI-first (target): AI reads Data Pack + Global Context, identifies problems directly, then backend validates and safely accepts output.

## What Stage 1 does

- Adds AI-first action taxonomy and categories in shared types.
- Adds feature flag `ACTIONS_AI_FIRST` with default `false` in runtime options.
- Adds contracts:
  - `ActionsAIFirstInput`
  - `ActionsAIFirstOutput`
  - `AIFirstDraftAction`
  - `AIFirstAffectedEntityRef`
  - `GlobalSemanticContext`
  - Global Semantic Context contracts (`GlobalSemanticContext`, validation)
- Adds AI-first validator (`ValidateActionsAIFirstOutput`).
- Updates frontend action types/categories/labels for new taxonomy.
- Adds tests for new contracts/validators and runtime env parsing.

## What Stage 1 does NOT do

- Does not switch `GenerateActions` to AI-first mode.
- Does not remove or rewrite `CandidateBuilder`.
- Does not remove legacy candidate-first files or prompts.
- Does not run AI-first generation in runtime.
- Does not run consistency reviewer in runtime (Stage 4 removed — see [removed-ai-consistency-reviewer.md](./removed-ai-consistency-reviewer.md)).
- Does not apply reviewer patches.
- Does not change `/api/v1/actions` response shape.
- Does not change DB schema.

## Feature flag

- `ACTIONS_AI_FIRST`
  - default: `false`
  - only read into `GenerateActionsOptions` in Stage 1
  - no runtime branching is introduced yet

## AI-first taxonomy

Allowed AI-first action types:

- `stock_replenishment`
- `stock_redistribution`
- `slow_turnover_review`
- `no_sales_with_stock`
- `sku_orders_drop`
- `key_sku_protection`
- `ad_high_drr_review`
- `ad_spend_no_orders`
- `ad_budget_reallocation`
- `funnel_weak_transition`
- `card_conversion_review`
- `category_underperformance`
- `sales_drop_review`
- `data_quality_issue`

Legacy aliases kept for candidate-first compatibility:

- `key_sku_risk`
- `ad_high_drr`

## Categories

AI-first categories:

- `stock`
- `slow_turnover`
- `sku`
- `sales`
- `advertising`
- `funnel`
- `category`
- `data_quality`

## AI-first input/output contracts

### `ActionsAIFirstInput`

- Root input for future generator.
- Anchored to `aidatapack.SellerAIDataPack`.
- Includes allowed taxonomy (`types/categories/priorities/cta`).
- Supports optional `GlobalSemanticContext`.

### `ActionsAIFirstOutput`

- Holds generated draft actions and optional notes.
- Contains `AIFirstDraftAction`, not persisted API items.

## Global Semantic Context contract

`GlobalSemanticContext` provides:

- `overall_story`
- primary/secondary semantic risks
- opportunities
- category priority order
- data limitations
- confidence and source fact links

This is contract-only at Stage 1.

## AI Consistency Reviewer (removed)

Stage 4 AI Consistency Reviewer was implemented briefly and removed from MVP due to high token cost. See [removed-ai-consistency-reviewer.md](./removed-ai-consistency-reviewer.md).

## Patch schema (removed)

Patch contracts (`AIConsistencyPatch`, `ValidateConsistencyPatches`) were removed with Stage 4 reviewer.

## AI-first validation rules

`ValidateActionsAIFirstOutput` validates:

- taxonomy allow-lists (`type/category/priority`)
- required fields (`title/reason/next_step/evidence_fact_ids/affected_entities`)
- fact grounding in `DataPack.FactRegistry`
- entity grounding in `DataPack.EntityRegistry` + special entities (`account:seller`, `funnel:account`, `data_quality:*`)
- forbidden claims and ad SKU-level claim guardrails
- numeric claims in seller-facing text against available facts
- CTA keys against allowed list (if provided)
- optional `operational_detail` evidence links

Legacy `source_candidate_ids` are optional here by design.

## Next stages

- Stage 2: connect AI-first generation path (behind flag).
- Stage 3: unified pipeline (Data Pack → Global Context → Dashboard → Actions).
- Stage 5: Dashboard Top-3 from Actions, UI/filter expansion, action-context chat.

Current architecture: [current-ai-first-architecture.md](./current-ai-first-architecture.md).
