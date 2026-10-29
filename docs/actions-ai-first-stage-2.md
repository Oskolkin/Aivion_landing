# Actions AI-first Stage 2

## Что меняет Stage 2

Stage 2 подключает новый runtime path генерации действий напрямую из Seller AI Data Pack.

- `ACTIONS_AI_FIRST=false` (default): работает legacy candidate-first pipeline.
- `ACTIONS_AI_FIRST=true`: работает AI-first pipeline:
  - Data Pack input
  - AI-first prompt
  - AI output validation
  - AI retry по validation issues
  - persist только accepted actions

## Feature flag

- `ACTIONS_AI_FIRST`
  - default `false`
  - при `true` включается `generateActionsAIFirst`
  - legacy path не удалён и не изменён по смыслу

## Legacy mode vs AI-first mode

### Legacy mode

- CandidateBuilder -> candidates -> domain prompts -> legacy validator -> persist.
- По-прежнему требуется `source_candidate_ids`.

### AI-first mode

- Seller AI Data Pack -> `ActionsAIFirstInput`.
- Модель сама выбирает:
  - проблему
  - type/category/priority
  - object_actions/action_plan/completion_criteria.
- Backend не передаёт candidates и не делает business ranking проблем.

## Как собирается `ActionsAIFirstInput`

Источник: `aidatapack.Service` (builder интерфейс `AIDataPackBuilder`).

В input передаются:

- `seller_account_id`, `metric_date`
- `data_pack_schema_version`, `data_pack_hash`
- `data_pack`
- `allowed_action_types` (активная Stage 2 taxonomy)
- `allowed_categories` (без `data_quality`)
- `allowed_priorities`
- `forbidden_claims`
- `max_actions`

## Active taxonomy Stage 2

### Categories

- `stock`
- `slow_turnover`
- `sku`
- `sales`
- `advertising`
- `funnel`
- `category`

`data_quality` не входит в active generation.

### Types

- `stock_replenishment`
- `stock_redistribution`
- `slow_turnover_review`
- `no_sales_with_stock`
- `sku_orders_drop`
- `key_sku_protection`
- `sales_drop_review`
- `ad_high_drr_review`
- `ad_spend_no_orders`
- `ad_budget_reallocation`
- `funnel_weak_transition`
- `card_conversion_review`
- `category_underperformance`

Не используются в active prompt:

- `data_quality_issue`
- `key_sku_risk`
- `ad_high_drr`

## Почему нет data_quality actions

`data_quality` остаётся внутренним/future направлением. Stage 2 ориентирован на user-facing исполнимые задачи для продавца.

## Почему нет open list / export / copy / snooze

Stage 2 считает действие самодостаточной задачей:

- объектный список внутри action detail (`object_actions`);
- шаги внутри action detail (`action_plan`);
- критерий выполнения (`completion_criteria`).

AI-first actions используют только 3 кнопки:

- Ask AI
- Mark completed
- Mark irrelevant

## AI-first prompt

Prompt требует:

- только факты из Data Pack;
- подтверждение чисел `evidence_fact_ids`;
- запрет размытых действий;
- запрет SKU-level ad claims;
- запрет price/margin/repricing утверждений;
- обязательные object_actions/action_plan/completion_criteria.

## Validation

`ValidateActionsAIFirstOutput` проверяет:

- allowed taxonomy;
- required fields self-sufficient action contract;
- existence of `evidence_fact_ids` и entities;
- numeric grounding;
- forbidden claims;
- advertising campaign-level guardrails;
- отсутствие disallowed CTA.

## AI retry (без silent repair)

Flow:

1. compose AI-first output;
2. validate;
3. если `repairable/invalid` -> retry prompt с `validation_issues`;
4. validate retry output;
5. persist только accepted actions.

Backend не переписывает смысловые поля action и не подменяет type/category/title/reason.

## Persist в существующую модель

AI-first drafts маппятся в `ActionItem`/`action_items`:

- сохраняются type/category/priority/status;
- сохраняются facts/entities;
- self-sufficient blocks сохраняются в `operational_detail` + payload;
- `source_candidate_ids` не требуются для AI-first persist guard.

Legacy persist guard остаётся строгим для candidate-first.

## Что остаётся на Stage 3

- Global Semantic Context generation/runtime integration.
- Единый Seller AI Pipeline orchestration.

## Stage 4 (removed) и Stage 5

- Stage 4 AI Consistency Reviewer удалён из MVP — см. [removed-ai-consistency-reviewer.md](./removed-ai-consistency-reviewer.md).
- Stage 5: Dashboard Top-3 из финальных Actions, Action detail, Ask AI, admin diagnostics.

Актуальная архитектура: [current-ai-first-architecture.md](./current-ai-first-architecture.md).
