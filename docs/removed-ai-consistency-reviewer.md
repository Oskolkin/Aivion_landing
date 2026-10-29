# Removed: AI Consistency Reviewer (Stage 4)

## Background

AI Consistency Reviewer was implemented as Stage 4 of the AI-first pipeline. It compared Dashboard and Actions outputs, proposed JSON patches, and persisted review artifacts.

## Why removed

For MVP, reviewer cost (extra large OpenAI call + patch validation/applier complexity) outweighed the benefit. The product target is a simpler unified pipeline:

```
Data Pack → Global Semantic Context → Dashboard → Actions
```

## What was removed

- Reviewer client, prompts, fake provider, patch applier, patch validator
- `ai_consistency_reviews` table and `consistency_*` columns on `ai_pipeline_runs`
- Reviewer devtool flags (`--reviewer`, `--reviewer-fake-mode`, etc.)
- Reviewer artifact kinds (`consistency_reviewer_*`, `final_*`, fingerprint artifacts)
- Env vars: `AI_CONSISTENCY_REVIEWER_ENABLED`, `AI_CONSISTENCY_REVIEWER_MAX_INPUT_TOKENS`

## Current pipeline

Stage 3 `SellerAIPipelineService` remains the unified orchestrator:

1. Build & validate Data Pack
2. Generate Global Context
3. Generate Dashboard (optional)
4. Generate Actions (optional)
5. Persist `pipeline_summary` without reviewer fields

## Stage 5

Dashboard Top-3 should be built from Actions output without a separate reviewer step.
