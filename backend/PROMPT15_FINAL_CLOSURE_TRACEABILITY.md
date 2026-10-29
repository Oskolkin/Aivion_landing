# Prompt 15 — Final Closure Traceability

Does not replace PROMPT150 / PROMPT151 / PROMPT152 / PROMPT153.

Date: 2026-09-03 (provider integrity: turn-global CallID + CatalogV1 submit/enum/required)  
Branch: `codex/canonical-sales-hardening`  
HEAD: `29fe417428ed8075b7c7480422d9bb1d04147529` (working tree dirty: provider integrity; not committed)

## Status

`PROMPT_15_FINAL_STATUS = CLOSED`

Prompt16 was not started. Prompt15.4 was not created.

## Run history (retained)

### Run 1 (prior archive)

```
backend Playwright 8/12
D–G FAIL — Actions fixture missing (analysis_publications empty for browser seller)
```

### Run 2 (prior closure pass)

```
canonical Action fixture implemented (TestE2EPreparePrompt15BackendScenario)
backend Playwright 2/12 (A, B)
first semantic failure = scenario C (GENERAL_GUIDANCE instead of E2E_STOCK=0)
D–L after C 90s timeout + next dev death = BLOCKED_BY_PREVIOUS_INFRA_FAILURE
mocked Playwright 10/11 (idempotent retry was not a real retry)
```

### Run 3 (entity search)

```
production: Factual API SearchEntities; Chat search_entities no longer List* pageSize=50
search integration: SKU/Campaign/Category after first 50, seller isolation, frozen cutoff, NL/numeric, P1/P2 tool preflight = PASS
Prompt15 PG family = PASS (~128s, marketplace_ai_prompt15_final_test, migration 93 dirty=false)
frontend lint/test/tsc/build = PASS
gofmt/vet/api+worker build = PASS
go test ./... = FAIL only unifiedanalysis/prompt (Windows EDR Access denied on prompt.test.exe); retry with -o tmp/promptpkg.test.exe = PASS
performance: 20 SKU batch=1 get=0 ~81ms; 200-turn history truncated; large search +400 SKU 6ms searchCalls=1 listSKU=0
mocked Playwright this pass = NOT GREEN (environment: Next on :3000 died / production :3010 tests hung on login without API)
backend-backed A–L this pass = NOT RUN (API process not started; C not re-executed in browser)
full ./internal/analysis/... on a new empty DB = NOT RUN this pass
```

### Run 4 (this pass — local stack)

```
local PostgreSQL marketplace_ai_e2e_prompt15_test version=93 dirty=false
local MinIO 127.0.0.1:19000 (API s3 smoke test ok)
go run ./cmd/api :8081 AI_PROVIDER_MODE=fake FAKE_PROVIDER_DELAY_MS=2000
frontend npm run start :3010 (not next dev)
P1/P2 tool preflight TestPrompt15SearchPolarisP1P2ToolPreflight = PASS (KNOWN 0 then 300)

mocked Playwright:
  npx playwright test e2e/prompt15-chat-v2.spec.ts
  E2E_SKIP_WEB_SERVER=1 E2E_BASE_URL=http://127.0.0.1:3010
  11 passed, failed=0, skipped=0, ~7.7s
  TEST DEFECT fixed: anti-double-submit no longer waits for Send while in-flight (button is Cancel)

backend-backed focused:
  C P1→P2 browser = PASS (assistant[0]=E2E_STOCK=0, latest=E2E_STOCK=300)
  Actions API preflight = PASS (after cookie secure:false)
  D handoff = PASS (heading locator; TEST DEFECT was strict getByText)
  E 4→3 = PASS
  F singleton = PASS
  G historical = PASS after fake reads ActionTurnContext + base_context publication_id
    (origin_pub=1 current_pub=6 recommended_price from origin parameters)

full A–L same stack:
  11 passed (preflight, A–J, H, I)
  2 failed: K, L export
  duration ~4.0m then extra K/L retries still FAIL

K/L evidence:
  UI: «Поток ответа завершился без результата»
  chat_turns status=VALIDATING error_code empty, no assistant_message_id
  chat_export_provenance count=0
  no download link
  TEST DEFECT also: locator expected /\.csv|выгруз|chat-export/ but UI link is «Скачать файл» (locator updated; still no artifact)
```

### Run 5 (focused export diagnostics + K/L fix)

```
K_FIRST_FAILING_STAGE = finalize.tool_audit_insert
exact internal error =
  finalize.tool_audit_insert: ERROR: duplicate key value violates unique constraint
  "uq_chat_turn_tool_calls_call" (SQLSTATE 23505)
classification = PRODUCTION DEFECT (FakeProvider) + SSE/finalization recovery gaps

Root cause:
  question "Выгрузи продажи по Polaris 417" matched both «выгруз» and «продаж»/Polaris
  branches; FakeProvider emitted two search_entities calls with the same CallID
  (fake-N-se). FinalizeSuccessfulTurn INSERT failed; turn left VALIDATING;
  HTTP handler discarded AskStream error (`_, _ = h.v2.AskStream(...)`) → silent SSE EOF.
  Frontend correctly showed «Поток ответа завершился без результата».
  Scripted PG export tests never hit this (`.Script(Final)` skips tools).
  MemoryArtifactStore tests did not prove Chat export MinIO.

Fixes:
  FakeProvider uniqueProviderToolCalls
  tool-audit ON CONFLICT (turn_id, call_id) DO NOTHING + in-TX dedupe
  export/finalize stage wrapping
  recoverFailedSuccessFinalization → detached FinalizeFailedTurn
    codes EXPORT_FINALIZATION_FAILED / FINALIZATION_FAILED
    seller copy «Не удалось подготовить файл выгрузки.»
  SSE handler tracks done/error; late errors emit event:error
  v2 download: ErrExportForbidden → legacy fallback; other errors fail closed

Focused tests (marketplace_ai_e2e_prompt15_test + local MinIO):
  CurrentPublicationExportDatasetPreflight PASS (rows=3 resolve=4.7ms)
  CurrentPublicationRealMinIOPrepare PASS (Put/Get/hash/Delete put=8.9ms)
  DynamicFakeExportStoredDownloadProvenance PASS (search_entities, batch=1, done=1)
  ExportFinalizationFailureBecomesDurableFailed PASS
  ExportFinalizationFailureCleansArtifact PASS
  ExportFinalizationFailpointMatrix PASS (incl. pre-commit FailCommit)
  StreamFinalizationErrorEmitsErrorEvent PASS (error=1 done=0)
  V2ExportDownloadDoesNotFallbackOnStorageFailure PASS
  LegacyInputAttachmentStillFallsBack PASS

Browser (npm run start :3010, API :8081, fake delay 2000ms, current_pub=6):
  K PASS (download CSV bytes>0, Polaris/9105 + metrics)
  L PASS (reload keeps assistant download link)
```

### Run 6 (full final regression)

```
backend-backed Playwright e2e/prompt15-chat-v2-backend.spec.ts
  E2E_CHAT_BACKEND=1 E2E_SKIP_SEED=1 E2E_SKIP_WEB_SERVER=1
  E2E_BASE_URL=http://127.0.0.1:3010 BACKEND_PROXY_URL=http://127.0.0.1:8081
  13 passed, failed=0, skipped=0, 1.2m  (preflight + A–L)

mocked Playwright e2e/prompt15-chat-v2.spec.ts
  12 passed (incl. SSE error ≠ silent EOF), failed=0, skipped=0, 11.2s

Prompt15 PG family TestPrompt15|TestPrompt152|TestPrompt153
  marketplace_ai_e2e_prompt15_test + MinIO
  PASS 138.719s

gofmt -l . empty; go vet ./... PASS; go build cmd/api + cmd/worker PASS
go test -p 1 ./... FAIL only unifiedanalysis/prompt prompt.test.exe Access is denied
  renamed tmp/promptpkg.test.exe --% -test.count=1 PASS

full ./internal/analysis/... -tags=integration -timeout 60m -p 1
  marketplace_ai_analysis_final_test migrated 1→93 dirty=false
  PASS 626.882s (analysis package 575.145s includes Prompt14)

frontend: lint 0 errors / 17 pre-existing warnings; unit PASS; tsc PASS; npm run build PASS

performance this pass:
  20 SKU batch=1 get=0 46.3ms
  20 categories batch=1 get=0
  200-turn history resolve=8.4ms truncated
  large entity search +400 SKU 5.5ms searchCalls=1
  export DatasetResolver 4.7ms rows=3; MinIO Put 8.9ms; dynamic AskStream 67ms
  N+1 = NO (demand/stock/sku batch=1 get=0)
  Chat-export storage timeout NOT added (K did not stall on Put/Get)

LIVE_OPENAI_SMOKE = NOT RUN (OPENAI_API_KEY empty)
NEW MIGRATION = NO
```

### Run 7 (tool-call identity + runtime schema — this integrity pass)

```
Re-audit classification:
  duplicate CallID silent finalizer drop = PRODUCTION DEFECT
  DisallowUnknownFields without Decode   = PRODUCTION DEFECT
  Playwright E after DONE (link remount / dual desktop+mobile testid) = TEST DEFECT
  FakeProvider unique CallID generation  = KEEP (not the runtime gate)

TOOL_CALL_IDENTITY_GATE
  duplicate CallID rejected before execution = PASS
  silent finalizer dedupe removed            = PASS
  seenCalls continue removed                 = PASS
  ON CONFLICT DO NOTHING removed             = PASS
  unique constraint uq_chat_turn_tool_calls_call preserved
  durable audit equals executed calls        = PASS (3==3)

RUNTIME_TOOL_SCHEMA_GATE
  CatalogV1 schema is runtime validator source of truth
  unknown property rejected = PASS
  missing required rejected = PASS
  wrong type rejected       = PASS
  bounds rejected           = PASS
  invalid enum rejected     = PASS
  invalid arrays rejected   = PASS
  trailing JSON rejected    = PASS
  forbidden backend-owned args rejected = PASS
  valid tools execute       = PASS (search_entities, get_sku_contexts; export K/L)

backend-backed Playwright (quiet stack, fresh prepare seller 271, pub 517):
  13 passed, failed=0, skipped=0, 1.1m  (preflight + A–L; E/K/L PASS)
  E test: wait for «3 активных» + aria-busy not true + action-ask-ai.first()
    (does not weaken origin=4 current_active=3 assertion)

mocked Playwright this pass (prior, same tree):
  12 passed, failed=0, skipped=0

Prompt15 PG family TestPrompt15|TestPrompt152|TestPrompt153
  marketplace_ai_e2e_prompt15_test + MinIO
  PASS ~139s (this pass family already green before integrity re-run;
  integrity tests + CategoryBatch/history reconfirmed PASS 13.6s)

gofmt -l listed Go files empty; go vet + api/worker build PASS (prior this pass)
go test -p 1 ./... FAIL only unifiedanalysis/prompt prompt.test.exe Access is denied
  renamed tmp/promptpkg.test.exe PASS

full ./internal/analysis/... -tags=integration -timeout 60m -p 1
  marketplace_ai_analysis_final_test 1→93 dirty=false
  PASS 503.307s (analysis package 490.255s includes Prompt14)

frontend lint/unit/tsc/build PASS (prior this pass; E2E spec-only after)

performance reconfirm:
  20 SKU / 20 category batch=1 get=0 (TestPrompt15CategoryBatchAndHistoryBounds PASS)
  200-turn history truncation still asserted in same test
  export DatasetResolver / K/L browser PASS
  schema validation is in-memory O(args), no DB

LIVE_OPENAI_SMOKE = NOT RUN
NEW MIGRATION = NO
```

### Run 8 (provider integrity — turn-global CallID + CatalogV1 submit/enum/required)

```
TURN_GLOBAL_CALLID
  duplicate CallID across rounds rejected before second Execute = PASS
  unique CallIDs across rounds accepted = PASS

SUBMIT_TURN_RESULT_SCHEMA
  unknown property rejected = PASS
  invalid answer_mode enum rejected = PASS
  wrong type rejected = PASS
  valid submit accepted (not executed as data tool) = PASS
  mixed submit + executable tools → INVALID_TOOL_CALL, exec=0 = PASS
  OpenAI plain-content Final fallback removed = PASS

ENUM_REPRESENTATION
  CatalogV1 []string enums normalized to []any
  schemaEnumValues([]any|[]string) = PASS
  invalid domain / export filter field rejected = PASS
  valid domain / export filters accepted = PASS

REQUIRED_EQUALS_CATALOG
  omitted required nullable rejected = PASS
  required nullable null accepted = PASS
  required nullable value accepted = PASS
  FakeProvider now emits limit/period nulls to match CatalogV1

Prompt15 PG family PASS 149.200s (e2e DB + MinIO)
backend-backed A–L 13 passed 1.4m (seller 345, current_pub 640; K/L PASS)
mocked Playwright 12 passed 10.0s
gofmt/vet/api+worker build PASS
go test -p 1 ./... FAIL only unifiedanalysis/prompt Access is denied
  tmp/promptpkg.test.exe PASS
full Analysis marketplace_ai_analysis_final_test 93 dirty=false PASS 514.239s
  (analysis package 504.038s includes Prompt14)
frontend lint 0 errors / unit / tsc / build PASS
NEW MIGRATION = NO
LIVE_OPENAI_SMOKE = NOT RUN
```

## K root cause (mandatory)

```
K_FIRST_FAILING_STAGE = finalize.tool_audit_insert
```

Sanitized underlying reason: duplicate `(turn_id, call_id)` on `chat_turn_tool_calls` from FakeProvider emitting the same `search_entities` CallID twice for an export+sales question. Not DatasetResolver, not MinIO Put, not provenance insert.

## Code this pass

### Production

- `get_action_context` now includes `current_publication_id` from frozen publication.
- `ActionMemberSnapshot` JSON tags so tool payloads expose `parameters` / `origin_parameters`.

### Fake provider (not live OpenAI)

- Historical: read in-process `*ActionTurnContext` plus `base_context.publication_id`.
- Export: ModeExport with empty grounding handles; detect «выгруз» from full request JSON.
- Deduplicate tool CallIDs (`uniqueProviderToolCalls`) so export+sales questions cannot insert duplicate `chat_turn_tool_calls`.

### Tests / E2E

- Mocked anti-double-submit: one in-flight POST; do not wait for Send to reappear.
- Backend spec: Actions heading; G scoped to last `chat-assistant-message`; K Playwright download + «Скачать файл».
- global-setup cookies `secure: false` so `APIRequestContext` accepts storageState.
- Fixture manifest includes `p1_universe_id` / `p2_universe_id`.
- Mocked SSE error event must show server copy, not silent EOF.

### Export / SSE (Run 6 verification pass)

- Stage wrap: `export dataset resolve` / `csv generate` / `storage put` / `attachment lookup` / `storage get`.
- Finalize stages wrap: begin, owner_fence, assistant_insert, export_attachment_insert, export_provenance_insert, tool_audit_insert, grounding_delete/insert, turn_complete, idempotency_complete, session_touch, commit.
- Success-finalization failure: cleanup artifact, then detached `FinalizeFailedTurn` (not the rolled-back TX). Ownership-lost is not overwritten.
- HTTP SSE no longer discards `AskStream` error; terminal `done`/`error` tracking; no JSON after SSE start.
- V2 download fails closed on storage errors; `ErrExportForbidden` still falls back to legacy/input.

### Tool call identity + runtime schema (Run 7)

- `ValidateProviderToolCalls` runs after provider return and before any `ToolExecutor.Execute`.
- Duplicate/empty CallID → `INVALID_TOOL_CALL`, executions=0, no success audit rows, SSE `error=1` `done=0`.
- Seller copy: «Не удалось корректно обработать запрос. Попробуйте ещё раз.»
- `FinalizeSuccessfulTurn` strict INSERT (no `seenCalls`, no `ON CONFLICT DO NOTHING`); empty CallID errors; unique constraint remains the DB backstop.
- `DecodeToolArgumentsJSON` (`UseNumber` + trailing EOF); OpenAI provider no longer configures a dead `DisallowUnknownFields` decoder.
- `ValidateToolArguments` uses `CatalogV1()` JSON Schema (`additionalProperties: false`, required, integer/enum/array bounds) plus `RejectForbiddenArgs`.
- `ToolExecutor.Execute` re-validates before incrementing `Executions`.
- FakeProvider `uniqueProviderToolCalls` kept; orchestration still rejects duplicates if a provider emits them.

### Provider integrity (Run 8)

- Agent tracks CallIDs for the whole Turn; a later round reusing an ID fails closed before that round executes.
- `ValidateProviderResponse`: XOR between `submit_turn_result`/`Final` and executable tools.
- OpenAI: every tool including `submit_turn_result` runs `ValidateToolArguments`; no unstructured content Final fallback.
- Enum validation accepts Catalog `[]any` and `[]string`; Catalog enums normalized to `[]any`.
- Required properties must be present; null is allowed only when the schema type includes null. Missing ≠ null.
- FakeProvider fills required nullable fields (`limit`/`period`/…) with JSON null.

## Remaining blockers before CLOSED

NONE

## OpenAI smoke

`LIVE_OPENAI_SMOKE = NOT RUN` (`OPENAI_API_KEY` not required; fake provider used).
