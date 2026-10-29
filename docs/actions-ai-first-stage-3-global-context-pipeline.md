# Actions AI-first Stage 3: Global Semantic Context & Unified Seller AI Pipeline

## Зачем нужен Global Semantic Context

Dashboard и Actions раньше могли генерироваться независимо: каждый путь сам строил Seller AI Data Pack и формировал свою «картину кабинета». Это приводило к расхождениям в приоритетах, формулировках и фокусе.

**Global Semantic Context** — внутренний (не seller-facing) смысловой слой между Data Pack и продуктовыми AI-выводами:

- `overall_story` — общая история кабинета;
- `primary_risks` / `secondary_risks` / `opportunities`;
- `priority_order` — порядок направлений;
- `data_limitations` — ограничения данных;
- `confidence` и `source_fact_ids`.

Global Context **не генерирует actions** и **не показывается пользователю**.

## Почему один Data Pack

Формула Stage 3:

```text
Seller AI Data Pack
  → Global Semantic Context
    → Dashboard generation
    → Actions generation (AI-first)
```

Один `data_pack_hash`, один `global_context_hash`, один `pipeline_run_id`, одна `metric_date`.

## SellerAIPipelineService

Пакет: `backend/internal/aipipeline`

Основной метод:

```go
func (s *SellerAIPipelineService) Run(ctx, req RunSellerAIPipelineRequest) (*RunSellerAIPipelineResult, error)
```

Шаги:

1. Проверка feature flags (`SELLER_AI_PIPELINE_ENABLED` + `ACTIONS_AI_FIRST`).
2. Создание `ai_pipeline_runs` (`running`).
3. **Один** build Seller AI Data Pack.
4. Валидация Data Pack + `data_pack_hash` (`sha256:...`).
5. Генерация Global Semantic Context (+ retry по validation issues).
6. `global_context_hash`.
7. Dashboard (если включён) с тем же pack/context.
8. Actions AI-first (если включён) из prebuilt input.
9. Связка `dashboard_run_id` / `actions_run_id` с pipeline run.
10. Debug artifacts (только при `DebugArtifacts=true`).

## Feature flags

| Flag | Default | Назначение |
|------|---------|------------|
| `SELLER_AI_PIPELINE_ENABLED` | `false` | Включает unified pipeline |
| `ACTIONS_AI_FIRST` | `false` | AI-first Actions (обязателен для unified pipeline) |
| `ALLOW_DEV_AI_PIPELINE` | `0` | Guard для devtools `run-ai-pipeline` |

Если `SELLER_AI_PIPELINE_ENABLED=true`, но `ACTIONS_AI_FIRST=false` → ошибка `seller_ai_pipeline_requires_actions_ai_first`.

Legacy paths (candidate-first Actions, standalone Dashboard narrative) **не меняются**, пока unified pipeline выключен.

## Таблицы

### `ai_pipeline_runs`

Хранит seller, metric_date, status, hashes, dashboard/actions run ids, error codes.

Статусы: `running`, `completed`, `completed_partial`, `failed`, `context_failed`, `dashboard_failed`, `actions_failed`, `empty`.

### `ai_pipeline_artifacts`

Debug JSONB artifacts по `pipeline_run_id` (только в debug mode).

Kinds: `data_pack`, `global_context_*`, `dashboard_*`, `actions_*`, `pipeline_summary`.

### Связи

- `ai_dashboard_narratives.ai_pipeline_run_id`, `data_pack_hash`, `global_context_hash`
- `action_generation_runs.ai_pipeline_run_id`, `data_pack_hash`, `global_context_hash`

## Global Context validation

`ValidateGlobalSemanticContext` проверяет:

- required fields (`schema_version`, `overall_story`, `confidence`, `source_fact_ids`);
- allowed/forbidden categories;
- grounding fact IDs и entities из Data Pack;
- unsupported numbers в тексте;
- forbidden claims (margin/profit/repricing/SKU-level ad/autopause).

Retry flow: generate → validate → retry with issues → validate again. Backend **не** делает silent semantic repair.

## Dashboard integration

`NarrativeRunOptions` принимает:

- `DataPackOverride`, `DataPackHash`
- `GlobalContext`, `GlobalContextHash`
- `PipelineRunID`, `ForceRegenerate`

`RunForSellerMetricDateWithOptions` — entrypoint для pipeline.

`generateViaDataPackChunks` не перестраивает pack, если передан override.

`DashboardSynthesisInput` включает `global_semantic_context`.

Freshness: при unified mode existing narrative current только если совпадают `data_pack_hash` и `global_context_hash`.

## Actions integration

- `BuildActionsAIFirstInputFromDataPack` — без DB/candidates rebuild.
- `GenerateActionsAIFirstFromInput` — generation из готового input.
- Prompt/validator учитывают `global_context` (soft warnings, не блокируют grounded actions).

## Devtool

```bash
SELLER_AI_PIPELINE_ENABLED=1 \
ACTIONS_AI_FIRST=1 \
ALLOW_DEV_AI_PIPELINE=1 \
go run ./cmd/devtools run-ai-pipeline \
  --seller-id=1 \
  --metric-date=2026-06-24 \
  --dashboard=true \
  --actions=true \
  --ai-enabled=false \
  --debug-artifacts=true
```

`--ai-enabled=false` использует deterministic fake providers для Global Context, Dashboard и Actions (без вызовов OpenAI).

## Stage 3 stabilization

### Offline / deterministic devtool mode

При `run-ai-pipeline --ai-enabled=false`:

- **Global Context** → `fake-global-context-deterministic`
- **Dashboard** → `FakeDashboardRunner` (`fake-dashboard-deterministic`)
- **Actions** → `fake-ai-first-deterministic`

Production `cmd/api` **не** подключает fake providers. Fake Dashboard доступен только через devtool/tests.

### Debug artifacts

При `--debug-artifacts=true` в `ai_pipeline_artifacts` сохраняются 11 kinds:

| Kind | Содержимое |
|------|------------|
| `data_pack` | Seller AI Data Pack |
| `global_context_input` | вход генерации Global Context |
| `global_context_prompt` | system/user/model prompt |
| `global_context_response` | финальный Global Context + raw |
| `global_context_validation` | результат валидации |
| `dashboard_input` | seller/date/hashes/global context summary |
| `dashboard_output` | fake/live dashboard payload summary |
| `actions_input` | ActionsAIFirstInput |
| `actions_output` | accepted actions summary |
| `actions_validation` | validation status/issues/counts |
| `pipeline_summary` | итоговый статус pipeline run |

Каждый artifact хранится в `payload_json` как:

```json
{
  "meta": { "pipeline_run_id": 3, "data_pack_hash": "sha256:...", ... },
  "payload": { ... }
}
```

### Семантика `artifact.hash`

- `artifact.hash` = `sha256:` canonical JSON hash **всего** `payload_json` документа (`meta` + `payload`).
- `data_pack_hash` и `global_context_hash` — scope metadata внутри `meta`, а не подмена `artifact.hash`.
- Исключение: для `data_pack` hash artifact может совпасть с `data_pack_hash`, если payload действительно равен Data Pack.

Helper: `aipipeline.HashArtifactPayload` / `aihash.HashJSON`.

## Failure modes

| Ситуация | Pipeline status |
|----------|-----------------|
| Data Pack build/validate failed | `failed` |
| Global Context failed | `context_failed` (Dashboard/Actions не запускаются) |
| Dashboard failed, Actions OK | `completed_partial` |
| Actions failed, Dashboard OK | `completed_partial` |
| Both failed | `failed` |

## Что Stage 3 НЕ делает

- Dashboard Top-3 из final Actions
- Seller-facing UI для hashes/artifacts
- data_quality actions, open-list/export/copy/snooze
- price/margin/repricing actions
- автодействия в Ozon

## Stage 5

- **Stage 5:** Dashboard Top-3 из Actions, deeper cross-product consistency, production orchestration hooks.

См. также: [removed-ai-consistency-reviewer.md](./removed-ai-consistency-reviewer.md) — Stage 4 AI Consistency Reviewer был удалён из MVP. Актуальная архитектура: [current-ai-first-architecture.md](./current-ai-first-architecture.md).

## Пакеты

- `backend/internal/aicontext` — neutral Global Semantic Context types
- `backend/internal/aihash` — stable `sha256:` hashes
- `backend/internal/aipipeline` — pipeline service, global context generation, store
