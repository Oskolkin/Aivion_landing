# Chat file attachments

## Supported formats (MVP)
`.pdf`, `.xlsx`, `.csv`, `.docx`, `.txt`, `.md`, `.json` (optional `.tsv`)

**Not supported:** legacy `.xls` — user message: save as `.xlsx` or `.csv`.

Blocked: executables, archives renamed as Office files, arbitrary binaries.

## Product limits
| Setting | Default |
|---------|---------|
| `CHAT_ATTACHMENT_MAX_FILE_BYTES` | 20 MiB |
| `CHAT_ATTACHMENT_MAX_TOTAL_BYTES` | 30 MiB |
| `CHAT_ATTACHMENT_MAX_FILES` | 5 |
| `CHAT_DRAFT_ATTACHMENT_TTL_HOURS` | 24 |
| `CHAT_OPENAI_FILE_TTL_HOURS` | 24 |
| `CHAT_ATTACHMENT_CLEANUP_ENABLED` | true |
| `CHAT_ATTACHMENT_CLEANUP_BATCH_SIZE` | 100 |

OpenAI hard limit (50 MiB) remains the upper bound.

## Flow
1. User picks file → `POST /api/v1/chat/attachments` (multipart)
2. File stored in MinIO artifacts bucket: `chat-attachments/<seller_account_id>/<uuid>` (original filename is display metadata only)
3. Row in `chat_attachments` with `attachment_kind=input`, `status=uploaded`
4. On manual **Send**, backend validates content (extension, MIME, magic bytes, ZIP structure for Office OpenXML), uploads to OpenAI Files API (`purpose=user_data`), passes `input_file` + text to Responses API
5. Attachments linked to message/session; `openai_file_id` stored until post-answer cleanup

## OpenAI file lifecycle
- After a successful answer (or failed ask path), backend attempts `DELETE /v1/files/{id}` via `AI_HTTPS_PROXY`
- On delete failure: `openai_cleanup_pending=true`; worker job `chat.attachments.cleanup` retries hourly
- Re-ask reuses MinIO copy and uploads a fresh OpenAI file if needed

## Proxy
- OpenAI Files API, model requests, and file delete use `AI_HTTPS_PROXY`
- Ozon API remains direct

## Spreadsheet limitation
If a sheet/CSV has more than 1 000 rows, `warning_code=spreadsheet_truncated_for_direct_ai_analysis` is stored and a user-visible warning is shown. The model must not claim full-file analysis.

## Cleanup
- **Draft orphans:** `message_id IS NULL`, `status IN (uploaded, failed)`, older than TTL → MinIO delete + optional OpenAI delete + `status=deleted`, `cleaned_at`
- **Schedule:** worker enqueues `chat.attachments.cleanup` every hour when `CHAT_ATTACHMENT_CLEANUP_ENABLED=true`
- **Devtools:** `go run ./cmd/devtools cleanup-chat-attachments --dry-run|--apply`

## Security
- Extension + declared MIME + content sniffing + Office OpenXML ZIP validation
- Safe download: authenticated `GET /api/v1/chat/attachments/{id}/download`, `X-Content-Type-Options: nosniff`, `Content-Disposition: attachment`
- Seller ownership on upload, delete, download, and ask
- No logging of file contents, API keys, or proxy credentials
