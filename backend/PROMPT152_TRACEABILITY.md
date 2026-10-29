# PROMPT 15.2 Traceability — Chat v2 Production Integrity & Closure

**Status:** `CODE_COMPLETE_VERIFY_PENDING`  
**Date:** 2026-09-02  
**Branch:** `codex/canonical-sales-hardening`  
**Migration head:** `000093_chat_v2` (no new migration)

Prompt150/151 remain historical reports. This document records Prompt15.2 corrective work and verification evidence.

---

## Closure matrix

| Gate | Exact proof | Real PG | Result |
|------|-------------|---------|--------|
| factual export contains actual rows | `DatasetResolver` + `TestPrompt15ExportStoredDownloadProvenance` asserts `ordered_units`, `sellable_stock`, not `context_mode` | YES | NOT RUN (DB schema incomplete) |
| exported file downloadable | Export download bytes/hash test | YES | NOT RUN |
| export attached to assistant message | `GetMessageRole` + `chat_attachments.chat_message_id` assertions | YES | NOT RUN |
| export provenance fail-closed | FK + storage fail tests | YES | NOT RUN |
| export orphan control | `ArtifactStore.Delete` on attachment/provenance failure | unit | PASS (code path) |
| strict OpenAI schemas | `TestOpenAIStrictCatalogSchemas` recursive nullable objects | local | PASS (13/13 tools) |
| durable tool audit | `CompleteTurnWithAudit` transaction | YES | NOT RUN |
| durable grounding | same transaction | YES | NOT RUN |
| failed-turn tool audit | `PersistToolCalls` on failed/cancel paths | YES | NOT RUN |
| durable CANCELLED | detached ctx + DB status assertion | YES | NOT RUN |
| Action Proposal parameters | `ActionMemberSnapshot.Parameters` from ActionView | YES | NOT RUN |
| origin Recommendation refs | `RecommendationRef` on MemberView/History | YES | NOT RUN |
| historical origin Publication | `OriginPublication()` not current pub | YES | NOT RUN |
| singleton origin Publication | case → `opened_by_publication_id` | YES | NOT RUN |
| P1 stock 0 / P2 stock 300 | existing Prompt15 tests | YES | NOT RUN |
| mixed attachment+cabinet | existing Prompt15 tests | YES | NOT RUN |
| canonical entity memory | existing Prompt15 tests | YES | NOT RUN |
| validation-before-streaming | existing Prompt15 tests | YES | NOT RUN |
| unknown-response retry | existing Prompt15 tests | YES | NOT RUN |
| required/optional factual failure | existing Prompt15 tests | YES | NOT RUN |
| category batch | existing Prompt15 tests | YES | NOT RUN |
| 20 SKU/category performance | unit counters only | YES | NOT RUN (PG timings) |
| 100/200-turn memory | unit + PG truncation test | YES | NOT RUN |
| full backend | go vet/build + chatv2 unit | — | PARTIAL PASS |
| full Analysis integration | blocked by local DB | YES | NOT RUN |
| Prompt14 regression | blocked by local DB | YES | NOT RUN |
| frontend build | npm run build | — | PASS |
| Playwright | prompt15-chat-v2.spec.ts | — | NOT RUN (webServer EACCES) |

`LIVE_OPENAI_SMOKE = NOT RUN` (`OPENAI_API_KEY` empty). Local strict schema conformance PASS.

---

## Production changes (15.2)

### Export
- **`export_dataset.go`**: typed `ExportFilterClause`, `DatasetResolver` calls `BatchGetSKUDemand` + `BatchGetSKUStock` + `BatchGetSKUs` on frozen context; CSV columns `entity_kind, entity_id, entity_label, metric, value, state, reason_code`; preserves KNOWN/UNKNOWN semantics (no fake zero).
- **`export.go`**: export bound to **assistant** message ID; compensating `Artifacts.Delete` + `DeleteExportAttachment` on DB/provenance failure.
- **`artifacts.go`**: `Delete` on all `ArtifactStore` implementations.

### Durable audit & cancellation
- **`store.go`**: `CompleteTurnWithAudit` (tool calls + grounding + turn completion in one transaction); `PersistToolCalls`, `DeleteExportAttachment`, `GetMessageRole`.
- **`service.go`**: success path uses audit transaction; failed/cancel paths persist tool calls; cancellation finalization uses **detached 5s context** (not cancelled request ctx).

### Action Chat
- **`ActionMemberSnapshot`**: `Parameters`, `RecommendationRef`, `Limitations`.
- **`actionview`**: recommendation ref in overlay SQL + history SQL; **`OriginPublication(viewID)`** from group publication or case `opened_by_publication_id`.
- **`service.go` ActionHandoff**: uses true origin publication, not current seller pointer.
- **`context.go` overlayAction**: copies full proposal context from ActionView.

### OpenAI strict schemas
- **`catalog.go`**: `export_dataset.filters` → typed array (no `additionalProperties: true`); nullable enums include `null`.
- **`openai_schema_test.go`**: recursive validation for `type: ["object","null"]` and nullable enums.

---

## Verification commands

```bash
# Unit (PASS)
go test ./internal/chatv2 ./internal/actionview

# PG (FAIL — local marketplace_ai_test hybrid schema: missing semantic_mapping_hash)
TEST_DATABASE_URL=postgres://postgres:postgres@127.0.0.1:55432/marketplace_ai_test?sslmode=disable \
  AI_PROVIDER_MODE=fake \
  go test -tags=integration -count=1 -timeout 25m -p 1 ./internal/analysis -run 'TestPrompt15|TestPrompt152'

# Frontend (PASS)
npm run lint && npm run test && npx tsc --noEmit && npm run build

# Playwright (NOT RUN)
npx playwright test e2e/prompt15-chat-v2.spec.ts
```

---

## Remaining blockers before CLOSED

1. **PostgreSQL**: CI-style clean migrated DB through `000093` (local `marketplace_ai_test` is hybrid ~174 tables + manually applied 90–93; analysis pipeline fails `semantic_mapping_hash` missing).
2. **Playwright**: Next webServer `EACCES` fetch in this environment.
3. **Full regression**: `go test -p 1 ./...` and analysis integration suite after (1).

---

## Final state

```text
Prompt 15   = CODE_COMPLETE_VERIFY_PENDING
Prompt 15.1 = CODE_COMPLETE_VERIFY_PENDING
Prompt 15.2 = CODE_COMPLETE_VERIFY_PENDING
```

Next: **Prompt16 — Evals / Observability** (NOT STARTED)
