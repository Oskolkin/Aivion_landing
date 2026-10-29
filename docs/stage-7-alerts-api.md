# Stage 7 Alerts API

MVP Alerts API exposes alert list/summary/actions and a synchronous manual run endpoint.

## Endpoints

- `GET /api/v1/alerts`
- `GET /api/v1/alerts/summary`
- `POST /api/v1/alerts/run`
- `POST /api/v1/alerts/{id}/dismiss`
- `POST /api/v1/alerts/{id}/resolve`

All endpoints are authenticated and scoped to current seller account from session context.

## `GET /api/v1/alerts`

Query params:

- `status`
- `group`
- `severity`
- `entity_type`
- `limit` (default `50`, max `200`)
- `offset` (default `0`)

Returns paginated alert items including lifecycle timestamps and parsed `evidence_payload` JSON.

## `GET /api/v1/alerts/summary`

Returns:

- `open_total`
- severity counters (`critical/high/medium/low`)
- `by_group` (`sales/stock/advertising/price_economics`)
- `latest_run` info from `alert_runs` (if exists)

## `POST /api/v1/alerts/run`

Request body (optional):

- `as_of_date` (`YYYY-MM-DD`)
- `run_type` (`manual|scheduled|post_sync|backfill`, default `manual`)

Runs alert engine synchronously for current account and returns run summary with per-group results.

## Actions

- `POST /api/v1/alerts/{id}/dismiss` -> marks alert as `dismissed`
- `POST /api/v1/alerts/{id}/resolve` -> marks alert as `resolved`

If alert is not found in current seller account scope, returns `404`.

## Out of scope

This step does not include frontend screens, background orchestration/jobs, recommendations, AI/chat, or notification delivery.
