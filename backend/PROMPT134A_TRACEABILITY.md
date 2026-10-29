# PROMPT №13.4A TRACEABILITY

## Status

`PROMPT_13_4A_STATUS = CLOSED` (assertion pass for 13.4A gates). **Current superseding closure:** `backend/PROMPT134B_TRACEABILITY.md`.

`PROMPT_13_STATUS = CLOSED` · `PROMPT_13_1_STATUS = CLOSED` · `PROMPT_13_2_STATUS = CLOSED` · `PROMPT_13_3_STATUS = CLOSED` · `PROMPT_13_4_STATUS = CLOSED` · `PROMPT_13_4B_STATUS = CLOSED`

Backend analysis pipeline **Stages 1–19 = CLOSED**.

Remaining backend blockers before Prompt 14: **NONE**

## Baseline (session)

| Item | Value |
|------|-------|
| branch | `codex/canonical-sales-hardening` |
| SHA | `e6d28dad060d3a43b764d935b0eb8f350211329b` |
| Go | `go1.26.1` / module `go 1.25.6` |
| working tree | uncommitted Prompt 13.4A assertion hardening (no commit requested) |
| latest migration | `000092_action_support_revalidation` |
| NEW_MIGRATION | **NO** |
| gofmt | `gofmt -l .` empty |

## Prompt 13.4 claim vs assertion gaps closed

| Gap | 13.4 actual | 13.4A proof |
|-----|-------------|-------------|
| Group member UPDATE | DELETE only | `UPDATE publication_action_group_members` rejected |
| Six-view identity | Overview seller/date; Chat date; others schema_version | envelope `publication_projections.publication_id` = P2 for all six + Overview identity fields |
| Concurrent loser residue | run-scoped count subset | seller snapshot + loser run residue 0 + outbox +1 winner only |
| Continuity Fact Handle | value 40, handle non-empty on P2 | P1 handle + P2 current handle + value 40 |
| Over-budget ErrorCode | empty code OR message contains | `ErrorCode == DEEP_PACK_TOO_LARGE` + no Deep output artifacts |
| P1 immutability | counts + content_hash | canonical SHA-256 of all immutable families |
| Large-row current-read | `WHERE publication_id=P2` counts | `GetCurrentPublication` then rows; `mix_count==0` |
| Outbox “business effect” | test-only aggregate map | Outcome B: no downstream production effect; duplicate process does not duplicate pubs/outbox |
| Test count | claimed 17 | actual `TestPrompt134*` = **15**; `TestPrompt134A*` = **9** |

## Production defects (minimal)

1. **Stage ErrorCode stored FailureClass, not domain code** — `runtime.failureCode` now persists `*deep.Error.Code` (e.g. `DEEP_PACK_TOO_LARGE`) while retry still uses FailureClass.
2. **Pack budget floor 1024 hid over-budget** when reserved output tokens exceeded remaining input — `EffectiveMaxInputTokens` no longer floors at 1024 (returns ≥1).

No contract version bumps.

## Outbox (Outcome B)

Production `ProcessOutbox` only marks `processed_at` / `attempts`. There is **no downstream production consumer**.

Test-only `TestOutboxSideEffectCount` / `TestOutboxProcessedAggregates` are **instrumentation only**, not a production idempotency mechanism.

Duplicate delivery proof (`TestPrompt134AOutboxDuplicateNoDownstreamEffect`):

```text
same outbox event processed twice
publications count unchanged
outbox row count unchanged
current publication unchanged
attempts incremented
```

## Mandatory acceptance matrix

| # | Gate | Required proof | Real PG | Test | Result |
|---|---|---|---|---|---|
| 1 | Group member UPDATE | SQL rejected | YES | `TestPrompt134ActionGroupImmutabilityPopulated` / `TestPrompt134AActionGroupMemberUpdateRejected` | PASS |
| 2 | Group member DELETE | SQL rejected | YES | same | PASS |
| 3 | Six-view Publication identity | all six = P2 envelope | YES | `TestPrompt134ASixViewPublicationIdentity` | PASS |
| 4 | Concurrent loser residue | full seller + run state | YES | `TestPrompt134AConcurrentFullLoserResidue` | PASS |
| 5 | Continuity P2 value | 40 current | YES | `TestPrompt134AContinuityFactHandles` | PASS |
| 6 | Continuity P2 handle | current Fact Handle | YES | same | PASS |
| 7 | Deep error exact | `DEEP_PACK_TOO_LARGE` | runtime | `TestPrompt134OverBudgetProviderCallsZero` | PASS |
| 8 | Deep provider calls | 0 | runtime | same | PASS |
| 9 | Deep artifact | absent | runtime | same | PASS |
| 10 | P1 content immutable | canonical hash equal | YES | `TestPrompt134APartiallySupportedP1ContentHash` | PASS |
| 11 | Stock current rows | P2 only, mix=0 | YES | `TestPrompt134ALargeRowCurrentReadAllFamilies` | PASS |
| 12 | Ads current rows | P2 only, mix=0 | YES | same | PASS |
| 13 | Focus current rows | P2 only, mix=0 | YES | same | PASS |
| 14 | Outbox duplicate contract | Outcome B | YES | `TestPrompt134AOutboxDuplicateNoDownstreamEffect` | PASS |
| 15 | Prompt134 test count | 15 | NO | `grep func TestPrompt134` | PASS |
| 16 | Status cleanup | no unlabeled VERIFY_PENDING | NO | search | PASS |
| 17 | gofmt | empty | NO | `gofmt -l .` | PASS |
| 18 | go vet | PASS | NO | `go vet ./...` | PASS |
| 19 | API build | PASS | NO | `go build ./cmd/api` | PASS |
| 20 | worker build | PASS | NO | `go build ./cmd/worker` | PASS |
| 21 | full go test | EDR-only exception | NO | `go test -p 1 ./...` | PASS* |
| 22 | full analysis integration | PASS | YES | `go test -tags=integration -p 1 ./internal/analysis/...` | PASS |

\* `internal/unifiedanalysis/prompt`: `prompt.test.exe Access is denied`. Equivalent `go test -c -o prompt_pkg_tests.exe` then run binary → **PASS**.

## Six-view evidence (sample run)

```text
P2 publication_id = 1437
seller = 1932
analysis_date = 2026-08-24
cutoff = 2026-08-25T12:00:00Z
universe = b330fa57c89be26b99d41a280eb9651a5f576afbb638930a51244b19f5d7a749

Overview     = P2 (envelope + OverviewV1DTO seller/date/cutoff/universe)
Funnel       = P2 (publication_projections.publication_id)
Stock        = P2
Advertising  = P2
Actions      = P2
Chat         = P2 (envelope + ChatContextDTO.analysis_date)
```

Official consumer contract: `GetCurrentPublication` + `GetProjection(publication_id, key)`. Domain bodies for Funnel/Stock/Advertising/Actions do not duplicate identity fields; identity is the projection envelope row.

## Continuity evidence (sample run)

```text
P1 value = 2
P1 Fact Handle = f_0001

P2 value = 40
P2 Fact Handle = ef_0019

AI-visible current value = 40
current grounding = P2 continuity-linked SKU_STOCK_HISTORY
stale current value substitution = NONE
```

Handles differ (`f_0001` vs `ef_0019`) as revision-aware evidence handles.

## Deep budget evidence

```text
ErrorCode = DEEP_PACK_TOO_LARGE
provider_calls = 0
accepted_attempt = NONE
Deep output artifact = NONE
```

## P1 immutable snapshot (sample run)

```text
P1 snapshot hash before P2 = 33403d7c04ba03ced507aac30670a7267ac97bc8ab78e96547bcbf135d69b779
P1 snapshot hash after P2  = 33403d7c04ba03ced507aac30670a7267ac97bc8ab78e96547bcbf135d69b779
equal = YES
```

Families: publication core, components, findings, recommendations, projections, stock/ad/focus rows, action groups/members, proposal parameters, proposal supports.

## Large-row current-read

Official path: `seller_current_publications` → `GetCurrentPublication` → `SELECT … WHERE publication_id = current.id`.

Sentinels: `P1_*:<p1.id>` vs `P2_*:<p2.id>` (publication_id on returned rows). HTTP Dashboard readers are Prompt 14.

```text
current publication = P2
P1 publication_id in current result = 0
P2 publication_id in current result = expected count
mix = 0
```

for Stock, Advertising, SKU Focus.

## Concurrent loser residue

```text
1 SUCCESS + 1 STALE_PUBLISH
loser run publications/components/findings/recs/projections/rows/proposals/assessments/groups/release = unchanged (0 extra)
seller publications = before+1 (winner only)
outbox = before+1 (winner only)
current pointer = winner
loser residue = 0
```

Seller A/B independence: `TestPrompt134ASellerLockIndependence` / `TestPrompt133SellerLockIndependence` — `pg_advisory_xact_lock`.

## Next step

Prompt 14 — Frontend / Dashboard + Actions migration — **NOT STARTED**
