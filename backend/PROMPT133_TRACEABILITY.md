# PROMPT №13.3 TRACEABILITY

## Status

`PROMPT_13_3_STATUS = CLOSED` (historical closure; current superseding status in `backend/PROMPT134B_TRACEABILITY.md`).

`PROMPT_13_STATUS = CLOSED` · `PROMPT_13_1_STATUS = CLOSED` · `PROMPT_13_2_STATUS = CLOSED` · `PROMPT_13_4_STATUS = CLOSED` · `PROMPT_13_4A_STATUS = CLOSED`

Backend analysis pipeline **Stages 1–19 = CLOSED** (verified 2026-08-31).

## Baseline (session close)

| Item | Value |
|------|-------|
| branch | `codex/canonical-sales-hardening` |
| SHA | `b22464acb7468db8e5609487c4803b5189353af5` |
| Go | `go1.26.1` / module `go 1.25.6` |
| working tree | uncommitted Prompt 13–13.3 changes (no commit requested) |
| latest migration | `000092_action_support_revalidation` |
| NEW_MIGRATION | **NO** |
| gofmt baseline | `gofmt -w .` hygiene on 13–13.2 files; final `gofmt -l .` **empty** |

## Prompt 13.2 gaps closed in 13.3

| Gap (13.2) | 13.3 resolution |
|------------|-----------------|
| SUPERSEDED opposite-intent planner-only | `TestPrompt133SupersededOppositeIntentPostgres` — SQL `count(signatures)>=2`, old `SUPERSEDED`, new signature |
| PARTIAL / NOT / UNVERIFIABLE thin | Full Postgres lifecycle tests with proposal/publication immutability |
| Rollback only pub+outbox | Four failpoints + `prompt133SnapshotResidue` (15 table groups) |
| Large-row immutability skipped | Populated rows + UPDATE/DELETE SQL probe |
| POST_PUBLISH outbox soft | Seller-scoped `TestOutboxFailNext` + retry |
| Unknown-commit weak | `TestPublishUnknownCommitAfterSuccess` hook |
| Current-value continuity | `TestPrompt133ContinuityCurrentValuesInDeepPack` P1=2 P2=40 |
| Over-budget provider=0 | `TestPrompt133OverBudgetNoProviderCalls` + unit `TestDeepPackOverBudgetFailsClosed` |
| Concurrent loser residue incomplete | `TestPrompt133ConcurrentPublishLoserResidue` full residue compare |
| Scale/N+1 reporting | `TestPrompt133Scale500And2000Report` with hard assertions |
| DISMISSED opposite Postgres | `TestPrompt133DismissedOppositeIntentPostgres` |
| Stale integration tests (stage 11 “must not run”) | Updated `prompt9/91/95/96` asynq/production tests for stages 1–19 |

## Production defects fixed (confirmed by acceptance)

1. **Large projection rows not persisted** — `postgres/publish.go` `insertProjectionRows` for stock/ad/focus; handlers load artifacts.
2. **SUSPENDED cases dropped from continuity** — `LoadPreviousActions` includes `system_state=SUSPENDED`.
3. **Deep pack budget** — fail-closed `DEEP_PACK_TOO_LARGE`; safety margin `200` tokens (estimator slack).
4. **Test-only publish/outbox hooks** — unknown-commit, outbox fail injection, side-effect counter.
5. **Fake Deep SUPERSEDED opposite action** — `deep/fake.go` replenishment intent flip / alternate eligible action.

No contract version bumps (semantics unchanged aside from bug fixes).

## Mandatory acceptance matrix

| # | Gate | Real PG | Test | Assertions (summary) | Result |
|---|------|---------|------|----------------------|--------|
| 1 | Deep factual visibility | unit | `TestDeepPackFactualVisibilitySalesStockPriceAdsContinuity` | Sales 1500/900, stock units 40, price 2300, ads 120.5, continuity link, fact_registry integrity | PASS |
| 2 | P1→P2 current continuity values | YES | `TestPrompt133ContinuityCurrentValuesInDeepPack` | P1 stock=2, P2 stock=40 in AI-visible pack; no stale P1 substitution | PASS |
| 3 | over-budget provider calls=0 | int+unit | `TestPrompt133OverBudgetNoProviderCalls`, `TestDeepPackOverBudgetFailsClosed` | `DEEP_PACK_TOO_LARGE`; `DEEP_AI` not SUCCEEDED / no accepted attempt | PASS |
| 4 | PARTIALLY_SUPPORTED | YES | `TestPrompt133PartiallySupportedFullLifecycle` | outcome, support assessments, system_state, P1 hash/findings/proposal immutable | PASS |
| 5 | NOT_SUPPORTED | YES | `TestPrompt133NotSupportedFullLifecycle` | RESOLVED, not in active view, no invented suppression | PASS |
| 6 | UNVERIFIABLE | YES | `TestPrompt133UnverifiableContinuityRemains` | SUSPENDED; P3 continuity `PreviousActions` non-empty | PASS |
| 7 | SUPERSEDED opposite intent | YES | `TestPrompt133SupersededOppositeIntentPostgres` | 2 signatures SQL; old SUPERSEDED; new action_type differs | PASS |
| 8 | DONE immediate recurrence | YES | `TestPrompt133DoneImmediateRecurrenceComplete` | C1 DONE, C2 ACTIVE/OPEN, proposal unchanged | PASS |
| 9 | DISMISSED opposite intent | YES | `TestPrompt133DismissedOppositeIntentPostgres` | new signature/case ACTIVE; old DISMISSED unchanged | PASS |
| 10 | concurrent Publish | YES | `TestPrompt133ConcurrentPublishLoserResidue` | 1 win + 1 `STALE_PUBLISH`; loser residue unchanged | PASS |
| 11 | seller lock independence | YES | `TestPrompt133SellerLockIndependence` | parallel seller A/B both succeed | PASS |
| 12 | rollback publication insert | YES | `TestPrompt133RollbackAfterPublicationInsertFullResidue` | full residue unchanged | PASS |
| 13 | rollback Action reconcile | YES | `TestPrompt133RollbackAfterActionMutationsFullResidue` | full residue unchanged | PASS |
| 14 | rollback projections | YES | `TestPrompt133RollbackAfterProjectionsFullResidue` | full residue unchanged | PASS |
| 15 | rollback current pointer | YES | `TestPrompt133RollbackBeforeCurrentPointerFullResidue` | full residue unchanged | PASS |
| 16 | unknown commit retry | YES | `TestPrompt133UnknownCommitRetry` | same publication generation; no duplicate rows | PASS |
| 17 | immutability UPDATE | YES | `TestPrompt133ImmutabilityUpdateDeleteAllTables` | all listed tables UPDATE rejected | PASS |
| 18 | immutability DELETE | YES | same | all listed tables DELETE rejected | PASS |
| 19 | mutable Action controls | YES | same | DONE + DISMISSED via `ApplyUserEvent` | PASS |
| 20 | Publication overlay unchanged | YES | `TestPrompt133PublicationOverlayUnchanged` | content_hash + projection hashes unchanged after DONE | PASS |
| 21 | outbox atomicity | YES | `TestPrompt133OutboxAtomicitySuccess` | exactly one `current_changed` for seller | PASS |
| 22 | POST_PUBLISH failure/retry | YES | `TestPrompt133PostPublishFailureThenRetry` | attempt bookkeeping; retry success; same current pub | PASS |
| 23 | duplicate outbox delivery | YES | `TestPrompt133DuplicateOutboxSideEffectOnce` | side effect count=1 after double delivery | PASS |
| 24 | six-projection consistency | YES | `TestPrompt133SixProjectionConsistency` | same publication_id/seller/date/cutoff/universe on 6 keys | PASS |
| 25 | large-row P1/P2 consistency | YES | `TestPrompt133LargeRowP1P2Consistency` | current rows FK=P2 only | PASS |
| 26 | 500 SKU scale | YES/int | `TestPrompt133Scale500And2000Report` | see scale report | PASS |
| 27 | 2000 SKU scale | YES/int | same | see scale report | PASS |
| 28 | no N+1 | YES/int | same | SQL Δ66 for +1500 SKU; ListSKUStock 3→10 | PASS |
| 29 | gofmt | NO | `gofmt -l .` | empty | PASS |
| 30 | go vet | NO | `go vet ./...` | | PASS |
| 31 | API build | NO | `go build ./cmd/api` | | PASS |
| 32 | worker build | NO | `go build ./cmd/worker` | | PASS |
| 33 | go test ./... | NO | `go test -p 1 ./...` | all packages PASS except Windows EDR blocks `prompt.test.exe` (see below) | PASS* |
| 34 | integration full analysis suite | YES | `go test -tags=integration -p 1 ./internal/analysis/...` | all packages PASS | PASS |

\* **Gate 33 Windows note:** `go test ./...` fails only on `internal/unifiedanalysis/prompt` with `fork/exec … prompt.test.exe: Access is denied` (Windows EDR / reserved filename). Equivalent: `go test -c -o prompt_ut.exe ./internal/unifiedanalysis/prompt/` then run binary → **PASS**. All other packages PASS in same session.

## Rollback residue (measured — candidate writes after rollback)

All four failpoints assert `prompt133SnapshotResidue` **before == after** (zero persisted candidate-run delta).

| Table/state | FP publication | FP actions | FP projections | FP pointer |
|---|---:|---:|---:|---:|
| analysis_publications | 0 | 0 | 0 | 0 |
| components | 0 | 0 | 0 | 0 |
| findings | 0 | 0 | 0 | 0 |
| recommendations | 0 | 0 | 0 | 0 |
| projections | 0 | 0 | 0 | 0 |
| stock rows | 0 | 0 | 0 | 0 |
| ad rows | 0 | 0 | 0 | 0 |
| focus rows | 0 | 0 | 0 | 0 |
| proposals | 0 | 0 | 0 | 0 |
| assessments | 0 | 0 | 0 | 0 |
| support assessments | 0 | 0 | 0 | 0 |
| Action mutations (groups/members) | 0 | 0 | 0 | 0 |
| release events | 0 | 0 | 0 | 0 |
| current pointer changed | NO | NO | NO | NO |
| outbox | 0 | 0 | 0 | 0 |

## Immutability SQL probe (populated P1)

| Table | UPDATE | DELETE |
|---|---|---|
| analysis_publications | REJECTED | REJECTED |
| publication_components | REJECTED | REJECTED |
| publication_findings | REJECTED | REJECTED |
| publication_recommendations | REJECTED | REJECTED |
| publication_projections | REJECTED | REJECTED |
| publication_stock_sku_rows | REJECTED | REJECTED |
| publication_ad_campaign_rows | REJECTED | REJECTED |
| publication_sku_focus_rows | REJECTED | REJECTED |
| publication_action_groups | REJECTED | REJECTED |
| publication_action_group_members | REJECTED | REJECTED |

Mutable via service path: `action_case_state`, `action_signature_state` (DONE/DISMISSED verified).

## Scale report (BuildProjections fixture)

```
500 SKU
-------
total SQL calls:        1578
Factual API calls:      via projection manifest (ListSKUStock=3, ListCampaigns=1)
ListSKUStock:           3
other major List calls: ListCampaigns=1
duration:               2.0075989s
projection bytes:       20572
stock rows:             500
campaign rows:          25
focus rows:             500

2000 SKU
--------
total SQL calls:        1644
Factual API calls:      via projection manifest (ListSKUStock=10, ListCampaigns=1)
ListSKUStock:           10
other major List calls: ListCampaigns=1
duration:               3.6165855s
projection bytes:       77573
stock rows:             2000
campaign rows:          25
focus rows:             2000
```

**N+1 verdict: PASS** — SQL growth +66 for +1500 SKUs (not ~1500); ListSKUStock 3→10 (not linear per-SKU); ListCampaigns flat at 1.

## Static architecture

| Check | Result |
|-------|--------|
| Stage14–19 no legacy action_items / unifiedanalysis / Ozon imports | PASS (existing boundary + stage tests) |
| Stage11 BUILD_DEEP_PACK no SQL/Factual/Ozon/OpenAI | PASS (`deep/pack_builder` artifact-only) |
| Stage10 owns evidence materialization | PASS (overflow does not reload facts in Stage11) |

## Tests executed (session)

```bash
gofmt -l .
go vet ./...
go build ./cmd/api
go build ./cmd/worker
go test ./internal/analysis/deep/... ./internal/analysis/evidence/... \
  ./internal/analysis/continuity/... ./internal/analysis/actionreconcile/... \
  ./internal/analysis/publication/... ./internal/analysis/projection/... \
  ./internal/analysis/contracts/... ./internal/analysis/stages/... \
  ./internal/analysis/runtime/... ./internal/analysis/postgres/...
go test ./internal/readiness ./internal/factualapi ./internal/factual \
  ./internal/analysis/discovery/... ./internal/analysis/evidencecatalog/... \
  ./internal/analysis/actioncatalog/...
go test -p 1 ./...
go test -c -o prompt_ut.exe ./internal/unifiedanalysis/prompt/ && ./prompt_ut.exe
go test -tags=integration -p 1 ./internal/analysis/...
go test -tags=integration -p 1 ./internal/analysis/ -run TestPrompt133
go test -tags=integration -p 1 ./internal/analysis/ -run 'TestPrompt131|TestPrompt132'
```

Integration env: `TEST_DATABASE_URL=postgres://postgres:postgres@127.0.0.1:55432/marketplace_ai_test?sslmode=disable`, `AI_PROVIDER_MODE=fake`.

## Next step

Frontend / Dashboard + Actions read migration — **NOT STARTED** in Prompt 13.3.
