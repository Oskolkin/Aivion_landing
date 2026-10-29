# AI Chat Guardrails

## Date picker removed

The AI chat UI no longer exposes a separate date or period control. Users should mention the period in natural language (for example: «за вчера», «за последние 7 дней», «за май»).

The API still accepts optional `as_of_date` (YYYY-MM-DD) for backward compatibility with older clients.

## Default period

When no period is provided in the request or question:

- **As-of date**: resolved by tools from the latest available dashboard metric date, or today (UTC) as fallback.
- **Analytics window**: last **7 days** (`DefaultChatDateRangeDays`) for tool date ranges when `date_from` / `date_to` are omitted.
- The planner user prompt includes the default window so the model can align tool calls.

Natural-language period parsing is not implemented yet; explicit dates in the question are interpreted by the planner.

## Pre-check (lightweight)

Before OpenAI is called, `Prechecker` evaluates the user message using three signal groups (combined, not any single rule):

1. **Marketplace keywords** — Ozon, sales, SKU, ads, metrics, alerts, etc.
2. **Seller conversation phrases** — e.g. «что у меня не так», «где я теряю деньги».
3. **Action keywords** — e.g. «покажи», «проверь», «почему».

Additional checks:

- **Export keywords** — parsed without OpenAI when type/format are clear (`export_execute`); see [ai-chat-exports.md](./ai-chat-exports.md). PDF and arbitrary SQL are refused; ambiguous type → clarification.
- **Entity hints** — SKU / offer_id / campaign ID; validated against the connected seller account when possible.
- **Clearly unsupported topics** — weather, recipes, poetry, politics, etc. — only rejected when there is no marketplace / action / conversation signal.

Principle: **reject only clearly unsupported queries**. Ambiguous seller questions are passed through or receive a clarification template.

## Controlled responses

| Reason | OpenAI called | User-facing behavior |
|--------|---------------|----------------------|
| `unsupported_topic` | No | Scope explanation + example questions |
| `unknown_sku` / `unknown_offer_id` / `unknown_campaign` | No | Entity not found in cabinet |
| `format_not_supported` | No | PDF requested; offer XLSX/CSV |
| `arbitrary_sql_refused` | No | User asked to run SQL |
| `export_type_not_supported` / `unsupported_by_source` | No | Unknown or unavailable export type |
| `clarification_needed` | No | Ask to narrow scope + examples |

Suggested rephrases are appended to the assistant message and stored in chat trace JSON (`precheck` payload).

## Traces

Controlled pre-check outcomes are stored under `tool_plan_payload.precheck` / `validated_tool_plan_payload.precheck` with:

- `user_message`, `precheck_result`, `is_supported`, `unsupported_reason`
- `detected_keywords`, `detected_entities`, `suggested_rephrases`
- `openai_called: false`

Rejected or controlled questions are logged at info level with `trace_id` and `seller_account_id`.

## Safety

- AI chat must not execute SQL or arbitrary backend actions.
- Export attachments (XLSX/CSV) are recognized but not produced in this MVP step.

## Code locations

- Pre-check: `backend/internal/chat/precheck.go`
- Entity validation: `backend/internal/chat/entity_resolver_sqlc.go`
- Integration: `backend/internal/chat/service.go` (`Ask`)
- Frontend chat UI: `frontend/components/chat-screen.tsx`
