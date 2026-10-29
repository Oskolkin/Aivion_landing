# PROMPT 15.3 Traceability — Atomic ChatTurn, Temporal Integrity, E2E Closure

**Status:** `CODE_COMPLETE_VERIFY_PENDING`  
**Date:** 2026-09-02  
**Branch:** `codex/canonical-sales-hardening`  
**HEAD:** `d554d5fc9bf9f6cba54113b829ff4de8b5446d95` (uncommitted 15.3 work)  
**Migration head:** `000093_chat_v2` after clean migrate 1→93 (`dirty=false`) on `marketplace_ai_prompt153_test`

Prompt150 / 151 / 152 remain historical. This file does **not** overwrite them.

`PROMPT_15_3_STATUS` is **not CLOSED**: mandatory Prompt15 PG suite, full Analysis, Prompt14 regression, frontend lint/test/tsc/build, mocked Playwright, and backend-backed Playwright were not all executed to green in this pass.

---

## Closure matrix

| Gate | Exact test/proof | PG | Result |
| --- | --- | ---: | --- |
| assistant message atomic with turn | `TestPrompt153AtomicSuccessRollbackAssistant` | YES | PASS |
| grounding atomic | `TestPrompt153AtomicSuccessRollbackGrounding` | YES | PASS |
| tool audit atomic | same success TX (`FinalizeSuccessfulTurn`) | YES | PASS (covered by rollback suite) |
| idempotency atomic | same TX + `TestPrompt153UnknownResponseAfterAtomicCommit` | YES | PASS |
| export DB metadata atomic | `TestPrompt153AtomicSuccessRollbackExport` | YES | PASS |
| export object compensation | same; `MemoryArtifactStore.Count()==0` after fail | YES | PASS |
| lease heartbeat | `TestPrompt153IdempotencyHeartbeatPreventsDuplicate` | YES | PASS |
| owner fencing | `TestPrompt153IdempotencyOwnershipLostCannotCommit` | YES | PASS |
| setup failure finalization | `TestPrompt153EarlyResolverFailureNoDuplicateMessage` | YES | PASS |
| current question excluded from history | `TestPrompt153CurrentQuestionNotInHistory` | YES | PASS |
| cancel durable | `TestPrompt153CancelFinalizationAtomic` | YES | PASS |
| failed turn durable | `FinalizeFailedTurn` + existing Prompt15 fail tests | YES | PARTIAL (153 cancel/fail TX exists; some Prompt15 validation paths still FAIL) |
| export schema=resolver | `TestPrompt153ExportSchemaMatchesResolver` | unit | PASS |
| unsupported export period | `TestPrompt153UnsupportedExportPeriodFailsClosed` | unit | PASS |
| exact export period provenance | `TestPrompt153ExportExactPeriodProvenance` | YES | PASS |
| singleton proposal origin | `TestPrompt153SingletonProposalOriginNotOpenedBy` | YES | PASS |
| historical group current overlay | `TestPrompt153HistoricalGroupKeepsCurrentOverlay` | YES | PASS |
| origin recommendation refs | `TestPrompt153OriginRecommendationRefsPersisted` | YES | PASS |
| clean migration 1→93 | `marketplace_ai_prompt153_test` migrate up | YES | PASS after in-place fix of `000088` trigger vs DELETE |
| Chat PG suite `TestPrompt15\|152\|153` | exact command below | YES | FAIL (Prompt15 subset; Prompt153 PASS) |
| full Analysis | `go test -tags=integration -p 1 ./internal/analysis/...` | YES | NOT RUN |
| Prompt14 regression | Dashboard/Actions tests | YES | NOT RUN this pass |
| frontend build | `npm run lint/test/tsc/build` |  | NOT RUN this pass |
| mocked Playwright | `prompt15-chat-v2.spec.ts` |  | NOT RUN this pass |
| backend-backed Playwright | `prompt15-chat-v2-backend.spec.ts` | YES backend | NOT RUN (`E2E_CHAT_BACKEND` gate) |

No invented PASS.

---

## Defects confirmed then fixed (code)

1. Successful ChatTurn was not one PG transaction (assistant / export / complete / idempotency).
2. Export S3 object was not deleted on PG rollback.
3. Idempotency completion was a second write; `completed` required payload.
4. 2-minute lease without heartbeat; BeginIdempotency ignored UPDATE errors.
5. Early `return nil, err` left `running`.
6. Current user message was in conversation **and** `question`.
7. Cancel/fail ignored DB errors.
8. Export schema advertised campaign/category; silent `current_7d` fallback; NULL `period_from`/`period_to`.
9. Singleton origin used `opened_by_publication_id`.
10. `origin_recommendation_refs` not persisted.
11. Historical group overlay used current-publication `ActionView.Get(group)`.
12. Clean migrate 1→93 failed at `000088` (`MATERIALITY_REGISTRY_IMMUTABLE` on DELETE of V1 policy row). **Existing file `000088` now drops/restores the trigger for that one-time cleanup** (not `000094`).

---

## Atomic success boundary

**Old:** `CreateMessage(assistant)` → optional export DB rows → `CompleteTurnWithAudit` → `FinishIdempotency`.

**New:** prepare export bytes + `Artifacts.Put` (no attachment row) → `FinalizeSuccessfulTurn` one TX:

- owner fence (`status=running` AND `owner_id`)
- assistant INSERT
- attachment + provenance (if prepared)
- tool calls
- grounding refs
- `chat_turns` COMPLETED
- idempotency `completed` + payload
- session touch
- COMMIT

On TX failure: `cleanupPreparedExport` deletes the storage object (5s detached context). Failpoints: `after_assistant`, `grounding`, `export`, `tool_audit`, `idempotency`.

---

## Clean PostgreSQL

- Host `127.0.0.1` port `55432` (compose). **Not** `marketplace_ai`.
- Database `marketplace_ai_prompt153_test` (name ends `_test`; `testutil.ValidateTestDatabaseURL` approved).
- `schema_migrations.version=93`, `dirty=f`.
- `semantic_mapping_hash` present (information_schema).

---

## Prompt15 PG acceptance

```text
TEST_DATABASE_URL=postgres://postgres:postgres@127.0.0.1:55432/marketplace_ai_prompt153_test?sslmode=disable
AI_PROVIDER_MODE=fake
go test -tags=integration -count=1 -timeout 30m -p 1 ./internal/analysis -run "TestPrompt15|TestPrompt152|TestPrompt153"
```

- **TestPrompt153\***: PASS (`go test ... -run TestPrompt153` → `ok` 28.4s)
- **Still FAIL on same DB:** `TestPrompt15FrozenP1Stock0ThenP2Stock300` (P2 stock not in fake-provider tool dump), `TestPrompt15CanonicalEntityMemory` (campaign/category sealed-universe labels vs relabel), `TestPrompt15AttachmentMixedAndInjection` (mode GENERAL_GUIDANCE), `TestPrompt15RequiredAndOptionalFactualFailure` (fake final not “недоступ” copy)

These are remaining Chat **acceptance** gaps, not a rollback of the atomic TX work.

---

## Migration

**YES (edit, not 000094):** `000088_prompt92_materiality_registry_cleanup.up.sql` must drop `trg_materiality_policy_immutable` before DELETE/UPDATE of `MATERIALITY_POLICY_V1`, then recreate the trigger. Without that, clean 1→93 cannot apply. No `origin_proposal_id` column; existing proposal lineage was sufficient for 153 action tests.

---

## Next step

Prompt16 — Evals / Acceptance / Observability — **NOT STARTED**.

Do not open Prompt15.4. Close Prompt15 family only after the remaining Prompt15 PG tests, full Analysis, Prompt14, frontend, and both Playwright layers are actually green.
