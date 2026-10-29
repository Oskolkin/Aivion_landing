# AI Chat — file exports (XLSX / CSV)

## Supported formats

- **XLSX** (`application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`)
- **CSV** (`text/csv; charset=utf-8`, UTF-8 BOM, semicolon delimiter for Russian Excel)

**PDF** is not supported. The user receives a controlled reply without calling OpenAI.

## How export is triggered

Explicit user phrases (выгрузи, скачай, сделай xlsx/csv, …) are parsed in `backend/internal/chat/precheck.go` via `chatexport.ParseExportRequest`. When type and format are known, the chat service runs a backend exporter **without OpenAI**.

If the export type is ambiguous, the user gets a clarification message (also without OpenAI).

## Export allowlist

| Export type | Implemented | Data source |
|-------------|-------------|-------------|
| `sales_summary` | yes | `daily_account_metrics` + ad spend |
| `sku_performance` | yes | `daily_sku_metrics` + critical SKU enrichment |
| `critical_sku` | yes | `analytics.CriticalSKUService` |
| `stock_risks` | yes | critical SKU / days of cover heuristics |
| `stock_surplus` | yes | `stock_analytics_snapshots` |
| `advertising_campaigns` / `campaign_performance` | yes | `analytics.AdvertisingService` |
| `alerts` | yes | open alerts (sqlc) |
| `recommendations` | yes | open recommendations (sqlc) |
| `orders_by_status` | yes | `orders` aggregated by status |

Planned / not in MVP: `product_funnel`, `returns`, `shipment_delay`, arbitrary SQL, competitor analysis, raw payloads, secrets.

## Security

- `seller_account_id` and `user_id` come only from auth/session.
- Export types must match the allowlist; no arbitrary columns or tables.
- **AI does not generate SQL**; **backend does not execute** user or AI SQL.
- File rows are built only from backend services/repositories.
- `storage_key` is never exposed to the frontend; downloads use `GET /api/v1/chat/attachments/{id}/download` with seller scoping.

### Download access control

- **Enforced:** authenticated seller context (`seller_account_id` from session cookie), attachment row scoped to that seller, chat session exists, assistant message belongs to the same session.
- **Not enforced:** per-user isolation inside a seller account — `created_by_user_id` on attachments is audit-only; any user authorized for the seller account may download (matches other seller APIs).
- **Never returned:** `storage_key`, raw MinIO paths.

## Export detection

- Explicit file verbs required (`выгрузи`, `скачай`, `сделай xlsx/csv`, `подготовь файл`, …). **«Покажи …»** alone stays in normal OpenAI chat.
- Ambiguous product-only requests (`выгрузи проблемные товары`, `сделай файл по товарам`) → clarification, no random export type.

## `stock_risks` vs `critical_sku`

- **`stock_risks`:** all SKUs on `as_of` date with depletion risk `medium+` (same rules as chat `get_stock_risks` tool), enriched with Ozon stock analytics when available.
- **`critical_sku`:** ranked key/problem SKUs via `CriticalSKUService` (separate export).

## S3 exports bucket

- Env: `S3_BUCKET_EXPORTS` (see `.env.example`, default `exports`).
- Local/docker: `docker-compose.yml` `minio-init` creates the `exports` bucket (`mc mb -p local/exports`).
- API startup validates bucket existence via `storage.New()`.
- If upload/download fails: assistant message explains storage is temporarily unavailable (no panic).

## Limits

- Max date range: **90 days** (default window: **7 days**, same as chat analytics default).
- Max rows per file: **10_000**.
- Empty period: file with **headers only** + message that there is no data.

## Storage

Binary files are stored in the S3/MinIO **exports** bucket under `chat-exports/{seller_account_id}/{attachment_id}/{file_name}`.

Metadata is in `chat_attachments` (migration `000030_create_chat_attachments`).

## Frontend

Assistant messages may include `attachments[]` with `download_url`, `file_name`, `mime_type`, `file_size`, `format`, `export_type`.
