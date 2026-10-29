# AI SKU Batch/Reducer Design (P1)

## Goal

Cover 100% SKU with AI analysis without sending full assortment in one prompt.

## Pipeline

1. Deterministic facts for all SKU are prepared by backend.
2. SKU universe is split into configurable batches (default 50).
3. AI analyzes each batch and returns strict structured JSON.
4. Batch findings are persisted.
5. Reducer aggregates findings into account/category-level outputs for dashboard and exports.

## DB proposal

### `ai_sku_analysis_runs`

- `id`, `seller_account_id`, `metric_date`, `status`
- `total_sku_count`, `analyzed_sku_count`
- `batch_total_count`, `batch_completed_count`, `batch_failed_count`
- `coverage_pct`, `prompt_version`, `model`
- `error_message`, `created_at`, `started_at`, `completed_at`

### `ai_sku_analysis_batches`

- `id`, `run_id`, `seller_account_id`, `metric_date`
- `batch_index`, `batch_total`, `status`, `sku_count`
- `batch_context_hash`, `input_context_json`
- `raw_ai_response`, `validated_payload`, `validation_errors`
- `prompt_version`, `model`
- `input_tokens`, `output_tokens`
- `error_message`, `created_at`, `started_at`, `completed_at`

### `ai_sku_findings`

- `id`, `run_id`, `batch_id`, `seller_account_id`, `metric_date`
- `sku`, `offer_id`, `product_id`, `category_id`, `category_name`
- `severity`, `problem_type`, `evidence`
- `recommended_action`, `confidence`, `created_at`

## Batch JSON schema (target)

```json
{
  "batch_summary": "string",
  "coverage": {
    "sku_count_received": 50,
    "sku_count_analyzed": 50
  },
  "sku_findings": [
    {
      "sku": "123456",
      "offer_id": "ABC-123",
      "severity": "high",
      "problem_type": "stock_risk",
      "evidence": "остатка на 4 дня при стабильных заказах",
      "recommended_action": "проверить поставку и дату следующего пополнения",
      "confidence": "high"
    }
  ],
  "category_signals": [],
  "batch_risks": []
}
```

Rules:

- AI references only SKU present in batch input.
- No issue for SKU -> no finding object.
- Empty findings are allowed, but `batch_summary` and `coverage` are mandatory.

## CLI proposal

```bash
go run ./cmd/dev-generate-sku-ai-analysis --seller-account-id=1 --metric-date=2026-06-17 --batch-size=50 --parallelism=2
```

Flags:

- `--seller-account-id`
- `--metric-date`
- `--batch-size`
- `--parallelism`
- `--force`
- `--dry-run`
- `--only-batches`
- `--only-reducer`
- `--inspect`

## MVP skeleton plan

1. Add migrations for three tables.
2. Add sqlc queries for run/batch lifecycle.
3. Add internal service with:
   - run planner
   - batch worker with retry
   - reducer
4. Keep feature flag OFF by default.

## Chat integration TODO

- `get_sku_ai_findings`
- `get_problem_skus`
- `get_ai_analysis_coverage`
