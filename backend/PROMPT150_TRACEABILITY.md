# PROMPT №15 TRACEABILITY

`PROMPT_15_STATUS = CODE_COMPLETE_VERIFY_PENDING`

Chat v2 owner path is implemented: frozen Publication/Factual turn context, typed read-only tools, model-native loop, grounding registry, validation-before-SSE, ActionView handoff, mixed attachments, factual export provenance.

Prompt16 / Prompt17 were **not** started.

---

## Architecture matrix

| Area | Legacy source | Chat v2 source | Active v2 uses legacy? |
| -------------------- | ---------------------- | -------------------------------- | ---------------------- |
| Publication context | planner SQL / daily_* | Current Publication + CHAT_CONTEXT/OVERVIEW/ACTIONS_SNAPSHOT | NO |
| SKU facts | tool_data_repository_sqlc | Factual API batch | NO |
| Stock | daily_sku_metrics | Factual API | NO |
| Advertising | unsafe_ads / ad_loss | Factual API | NO |
| Actions | action_key / action_items | ActionView / Cases overlay | NO |
| Attachments | attachment-only tool suppress | turn attachment context + cabinet tools | NO |
| Export | chatexport DataService SQL | Chat DatasetRef + FactualExporter + chat_export_provenance | NO |
| Conversation history | visual-only | ChatMemoryPolicyV1 backend selection | NO |
| AI planning | planner.go + answerer | ChatAgentProvider tool loop | NO |

## DB

Migration: `000093_chat_v2`

New tables: `chat_turns`, `chat_turn_contexts`, `chat_turn_tool_calls`, `chat_turn_grounding_refs`, `chat_action_bindings`, `chat_action_binding_members`, `chat_export_provenance`

Indexes: session/seller started, unique user_message, unique tool call_id, unique grounding handle, unique binding session + fingerprint, members unique (binding, case)

FKs: sessions, messages, publications, seller_accounts, attachments

Legacy retained: `chat_sessions.action_key` (Prompt17)

New column: `chat_attachments.completeness_status`, idempotency `turn_id`/`binding_id`

## Policies

| Policy | Version | Defaults |
|--------|---------|----------|
| result schema | `chat_turn_result_v1` | answer, answer_mode, grounding_handles, limitations |
| prompt | `chat_prompt_v1` | |
| model | `chat_model_policy_v1` | not a hardcoded model ID |
| tools | `chat_tool_policy_v1` | 6 rounds, 12 calls, 4 parallel, 20 entities, 50 rows/tool, 200 total rows, **90 days**, 32k result, 80k context |
| memory | `chat_memory_policy_v1` | 12 recent turns, 24k history bytes, 32 entities, 8k/4k attachment |
| retry | `chat_answer_retry_policy_v1` | 2 attempts, same frozen context |
| catalog | `chat_tool_catalog_v1` | see tools below |
| chat projection | `chat_context_projection_v1` reused (no silent v2 reinterpret) |

## Tool catalog

`get_publication_overview`, `list_publication_findings`, `list_current_actions`, `get_action_context`, `search_entities`, `get_sku_contexts`, `get_category_contexts`, `get_campaign_contexts`, `get_account_domain_facts`, `list_sku_metrics`, `list_category_metrics`, `list_campaign_metrics`, `submit_turn_result`

No `seller_id` / universe / cutoff args. No `run_abc_analysis` (canonical ABC not approved). No SQL/web/write tools.

## Owner path

`POST /api/v1/chat/ask` and `/ask/stream` require Chat v2 (`ChatHandler.WithV2`). No planner fallback.

`POST /api/v1/chat/action-handoffs` resolves `action_view_id` via ActionView.Get, persists canonical binding.

Physical files `planner.go` / `tool_data_repository_sqlc.go` remain until Prompt17; `internal/chatv2` architecture tests forbid importing them.

## Status at write time

Verification commands were being executed after this file. Treat CLOSED only if the report below records all gates PASS.
