# Unified Analysis Contracts v1

Этот документ фиксирует **frozen** контракты Unified Analysis Pack / Output.
Этап 1 зафиксировал схемы; этап 2 реализует Pack Builder поверх них.

## Frozen vs Stage 2 / Stage 3+

### Frozen

- schema versions (`aivion_analysis_pack_v1`, `aivion_unified_analysis_v1`);
- pack file list + stable ZIP presence (exactly 12 files);
- CSV column names/order;
- unified output structure;
- action enums (types/categories/priorities);
- null semantics catalog;
- Analysis Pack target daily history = **30 days**;
- Dashboard/Actions ownership mapping.

### Stage 2 (реализовано)

- `analysisdata.LoadWithOptions` (legacy 14d / unified 30d);
- формулы CSV и writers;
- pack hash / deterministic ZIP / technical validator;
- MinIO content-addressed storage;
- devtool `build-unified-analysis-pack`.

Подробности builder: [`unified-analysis-pack-builder.md`](./unified-analysis-pack-builder.md).

### Stage 3 (реализовано — provider smoke)

- OpenAI Containers / Container Files / Code Interpreter / background Responses;
- orchestration `unifiedanalysis.Service` (`mode=unified_analysis`);
- asynq tasks `unified_analysis.start|poll|cleanup|cancel`;
- technical smoke prompt `unified_provider_smoke_v1` (no Dashboard/Actions);
- devtools: `run-unified-provider-smoke`, `inspect-unified-provider-run`, `cancel-unified-provider-run`, `check-unified-provider-readiness`.

Подробности provider: [`unified-analysis-provider.md`](./unified-analysis-provider.md).

### Stage 4 (реализовано — analyst + technical validation)

- production prompt `unified_analyst_v1_5` + strict Structured Output
  `aivion_unified_analysis_v1`;
- asynq task `unified_analysis.validate_output`;
- technical output validator → `publication_ready` / `publication_blocked`;
- migration `000055` (unique index excludes terminal `publication_*`);
- devtools: `run-unified-analysis`, `inspect-unified-analysis-output`,
  `validate-unified-analysis-output`, `check-unified-analysis-readiness`,
  `run-unified-analysis-evals` (seller 9042).

Подробности analyst: [`unified-analysis-analyst.md`](./unified-analysis-analyst.md).

**Stage 4 boundary:** ends at `publication_ready`; no Dashboard/Actions
persistence, publication, frontend, post-sync, Reviewer, legacy deletion.

### Stage 5+ (не реализовано)

- publication Dashboard+Actions;
- seller-facing API / UI;
- post-sync auto-run; AI Reviewer.

## 1. Назначение `AnalysisDataSnapshot`

`backend/internal/analysisdata.AnalysisDataSnapshot` — общий внутренний снимок
сырых данных кабинета продавца:

- загруженные строки `dbgen` (metrics, products, stock, ads, funnel, …);
- вычисленные окна `DataWindows`;
- технический `EnrichmentSource`.

Snapshot **не** содержит Signals / Candidates / Global Context / Dashboard / Actions.

## 2. Границы ответственности loader

`analysisdata.SQLLoader`:

- валидирует `seller_account_id > 0`;
- UTC date-only `metricDate`;
- **`Load()`** — legacy default: 14d (`LegacyHistoryDays`);
- **`LoadWithOptions(HistoryDays)`** — shared orchestration; Unified Pack uses 30d;
- выбирает account funnel rows (детерминированная сортировка по дате);
- enrichment source;
- записывает `LoadedHistory` (игнорируется legacy `BuildPack`).

Legacy `SellerAIDataPack` продолжает использовать 14d окна и не меняет canonical hash.
Analysis Pack v1 использует 30d raw history; колонки `*_30d` заполняются builder'ом.

## 3. Файлы Analysis Pack v1

Версия: `aivion_analysis_pack_v1`

**Решение:** все файлы всегда присутствуют в ZIP (`Required=true`).

При отсутствии данных CSV:

- присутствует;
- содержит только header;
- `row_count = 0`;
- причина в `limitations.json`;
- availability отражается в manifest.

| File | Kind |
| --- | --- |
| `manifest.json` | json |
| `data_dictionary.json` | json |
| `analysis_policy.json` | json |
| `limitations.json` | json |
| `account_summary.csv` | csv |
| `account_daily.csv` | csv |
| `sku_summary.csv` | csv |
| `sku_daily.csv` | csv |
| `campaign_summary.csv` | csv |
| `campaign_daily.csv` | csv |
| `campaign_sku.csv` | csv |
| `product_enrichment.csv` | csv |

## 4. Колонки CSV

Порядок зафиксирован в `pack_columns_v1.go`. `row_id` первый, snake_case, без дублей.

`*_30d` колонки остаются в `sku_summary.csv`; значения null до расширения history на этапе 2.

## 5. Null semantics

См. `MetricNullSemanticsV1` в `null_semantics_v1.go`.

Кратко:

| Case | Rule |
| --- | --- |
| Observed zero | `0` |
| Unknown / unavailable source | `null` |
| Division by zero (`drr_pct`, `*_delta_pct`, conversions) | `null` |
| Not applicable | `null` + limitation |
| `*_30d` before stage-2 history | `null` |

## 6. Версии

| Contract | Version |
| --- | --- |
| Legacy Data Pack | `ai_data_pack_v1` |
| Analysis Pack | `aivion_analysis_pack_v1` |
| Unified Output | `aivion_unified_analysis_v1` |

Policy: `max_actions=20`, `max_top_actions=3`.

## 7. Unified output

Top-level: `schema_version`, `analysis_meta`, `analysis_outcome`, `dashboard`, `actions`, `limitations`.

Outcomes: `material_actions_found` | `no_material_actions` | `insufficient_data` | `analysis_failed`.

`actions` может быть `[]` для `no_material_actions` / `insufficient_data` /
`analysis_failed`.

**`analysis_failed` (frozen):** AI возвращает, когда pack технически присутствует,
но анализ невозможен по задокументированным причинам целостности данных.
Требования: `actions=[]`, пустые `top_action_ids` / `recommended_action_ids`,
непустой `limitations`. Технически валидный `analysis_failed` может дойти до
`publication_ready`.

**Не путать с provider failures:** refusal / incomplete / невалидный JSON от
провайдера → run `failed`, без AI-outcome `analysis_failed`. Ошибки contract
validator (unknown SKU, meta mismatch, …) → `publication_blocked`.

**`entity_ids` (scope-conditional, backend-enforced):** JSON Schema strict
subset не может выразить "minItems зависит от `scope_type`", поэтому
`scope.entity_ids` остаётся required array в схеме для всех scope. Условие
(sku/campaign scope ⇒ ≥1 элемент, существующий в `PackReferenceIndex`;
account scope ⇒ допускается `[]`) проверяет технический validator
(`output_missing_scope_entities` / `output_unknown_sku` /
`output_unknown_campaign` / `output_duplicate_sku_across_actions`), не схема.
Подробности: [`unified-analysis-analyst.md`](./unified-analysis-analyst.md#entity-ids-scopeentity_ids).

**`limitations`:** коды берутся только из `limitations.json` / фиксированного
allowlist; произвольный free-text считается невалидным.

Без DB IDs, pipeline IDs, debug, CTAs, provider metadata, candidate/fact IDs, global context hash.

## 8. Action enums

Active AI-first taxonomy only (`actions.ActionsAIFirstActive*`).

### Types

`stock_replenishment`, `stock_redistribution`, `slow_turnover_review`, `no_sales_with_stock`,
`sku_orders_drop`, `key_sku_protection`, `ad_high_drr_review`, `ad_spend_no_orders`,
`ad_budget_reallocation`, `funnel_weak_transition`, `card_conversion_review`,
`category_underperformance`, `sales_drop_review`

### Categories

`stock`, `slow_turnover`, `sku`, `sales`, `advertising`, `funnel`, `category`

### Priorities

`critical`, `important`, `watch`

### Explicit exclusions

| Value | Why excluded |
| --- | --- |
| `key_sku_risk` | legacy candidate type |
| `ad_high_drr` | legacy candidate type |
| `data_quality_issue` / `data_quality` | internal, not user-facing v1 |
| `growth` | unsupported by backend/frontend |

Frontend (`frontend/lib/actions-types.ts`) supports the same priorities/categories/types set for active filters.

## 9. Dashboard coverage matrix

| Dashboard block | AI-owned | Backend-owned | Removed | Contract field |
| --- | ---: | ---: | ---: | --- |
| Account summary health/status | yes | adapter | no | `dashboard.account_summary.status/health_score/...` |
| Headline | yes | adapter | no | `dashboard.account_summary.headline` |
| Executive summary | yes | adapter | no | `dashboard.account_summary.executive_summary` |
| Risk drivers | yes | adapter | no | `dashboard.account_summary.risk_drivers` |
| What changed | yes | adapter | no | `dashboard.what_changed` |
| Category summaries | yes | adapter | no | `dashboard.category_summaries` |
| Sales funnel | yes (narrative + stage values from pack) | numeric may be verified | no | `dashboard.sales_funnel` |
| Funnel analysis | yes | adapter | no | `dashboard.funnel_analysis` |
| Stock coverage | yes | adapter | no | `dashboard.stock_coverage` |
| Ad efficiency | yes | adapter | no | `dashboard.ad_efficiency` |
| Risk distribution | yes | adapter | no | `dashboard.risk_distribution` |
| Seller pulse narrative | yes | KPI numbers may be backend-verified | no | `dashboard.seller_pulse` |
| KPI cards | yes (labels/copy) | values may be backend-verified | no | `dashboard.seller_pulse.kpis` |
| SKU focus | yes | adapter | no | `dashboard.sku_focus` |
| Top Actions | yes (ids only) | resolve to TopActionItem | no | `dashboard.top_action_ids` |
| Recommended Actions | yes (ids only) | resolve to ActionItem | no | `dashboard.recommended_action_ids` |
| Limitations | yes | merge with pack limitations | no | `limitations` |
| Pipeline/debug/freshness/CTAs | no | backend runtime | no (not in AI contract) | — |

## 10. Action mapping

| Unified action | Current ActionItem |
| --- | --- |
| `analysis_action_id` | backend creates `action_key` |
| `type/category/priority` | same fields |
| `title/summary` | same |
| `problem + why_now` | `Reason` |
| first step | `NextStep` |
| `expected_effect` | `ExpectedEffect` |
| `scope/steps/completion_*` | AffectedEntities / OperationalDetail / CompletionConditions |
| `limitations/confidence` | Limitations / payload |

Adapter не реализован на этапе 1.

## 11–12. Backend checks (future) / non-checks

Technical contract only on later stages: schema, enums, id uniqueness, top_action_ids ⊆ actions, max counts, pack file presence.
No semantic business validation of AI narrative on stage 1.

## 13. Deferred to stage 2

ZIP, CSV writers, 30d SQL expansion, hash, MinIO, OpenAI, runtime, publication, frontend.

## 14. Open implementation questions (stage 2 only)

1. Exact CSV formulas / rounding.
2. Canonical hash algorithm.
3. ZIP canonicalization details.
4. MinIO key layout.
5. How `AnalysisDataSnapshot` carries 30d without changing legacy 14d pack windows.
