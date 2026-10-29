# Stage 13 — Ingestion downstream recalculation

## Overview

After Ozon **import jobs** finish and the parent **sync_job** transitions to `completed`, the worker enqueues a bounded **post-sync recalculation** task (`ozon.post_sync_recalculation`). That task:

1. Rebuilds **daily_account_metrics** for the seller over the last **30 UTC calendar days** (inclusive of today).
2. Rebuilds **daily_sku_metrics** over the same window.
3. Runs the **alerts engine** with `run_type = post_sync` and `as_of_date = today` (UTC calendar day).

Manual **admin** actions (`rerun-metrics`, `rerun-alerts`) and dev CLIs remain unchanged and can still be used for broader or ad-hoc rebuilds.

## What is intentionally not automated

**AI recommendation generation** is **not** triggered after ingestion. It stays **manual / admin / user-triggered** to control OpenAI cost and quality gates (see `stage-8-ai-recommendation-engine-scope.md`).

## Idempotency and retries

- **Metrics**: rebuild methods delete rows in the window then upsert from sources; repeating the task is safe.
- **Enqueue**: `asynq.Unique(48h)` + stable `TaskID` keyed by `sync_job_id` avoids duplicate queued work if the import task retries after a successful enqueue.
- **Alerts**: existing fingerprint / run lifecycle logic in `alerts.Service` applies for `post_sync` like other run types.

## Failure behaviour

If **metrics** fail, the post-sync task errors and **asynq retries** the whole task. If **metrics** succeed and **alerts** fail, the task errors so a retry can re-run alerts (metrics step remains idempotent).

## Code map

| Piece | Location |
|-------|----------|
| Task type + payload | `internal/jobs/asynq.go` |
| Enqueue on `sync_job` completed | `cmd/worker/main.go` + `WithSyncJobCompletedHook` on `OzonImportHandler` |
| Finalize detects first transition to `completed` | `internal/jobs/sync_job_finalize.go` → `ParentSyncFinalizeResult.SyncJobJustCompleted` |
| Handler | `internal/jobs/recalculate_after_sync.go` |
