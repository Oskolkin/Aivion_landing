# PROMPT №13.4 TRACEABILITY

## Status

`PROMPT_13_4_STATUS = CLOSED` (see `backend/PROMPT134B_TRACEABILITY.md` for current superseding closure).

`PROMPT_13_STATUS = CLOSED` · `PROMPT_13_1_STATUS = CLOSED` · `PROMPT_13_2_STATUS = CLOSED` · `PROMPT_13_3_STATUS = CLOSED` · `PROMPT_13_4A_STATUS = CLOSED`

Backend analysis pipeline **Stages 1–19 = CLOSED** (verified 2026-09-01).

## Baseline (session close)

| Item | Value |
|------|-------|
| branch | `codex/canonical-sales-hardening` |
| SHA | `55c24fec9057acd95c29dbb8322ff9a103668a47` |
| Go | `go1.26.1` |
| working tree | uncommitted Prompt 10.1–13.4 changes (no commit requested) |
| latest migration | `000092_action_support_revalidation` |
| NEW_MIGRATION | **NO** |
| gofmt baseline | `gofmt -l .` **empty** after final `gofmt -w` |

## Prompt 13.3 gaps closed in 13.4

| Gap (13.3) | 13.4 resolution |
|------------|-----------------|
| SUPERSEDED planner-only / wrong action type | `TestPrompt134SupersededOppositePriceIntentPostgres` — same `SKU_ADJUST_PRICE`, `DECREASE→INCREASE`, two signatures, Postgres rows |
| DISMISSED opposite intent thin | `TestPrompt134DismissedOppositePriceIntentPostgres` — suppression bound to old signature only |
| Continuity current value weak | `TestPrompt134ContinuityCurrentStockValues` — P1=2, P2=40 in continuity-linked stock evidence |
| Over-budget without provider_calls=0 | `TestPrompt134OverBudgetProviderCallsZero` — literal `provider_calls=0` |
| Concurrent loser subset residue | `TestPrompt134ConcurrentPublishFullLoserResidue` — full `prompt134SnapshotResidue` |
| Unknown-commit duplicate audit partial | `TestPrompt134UnknownCommitFullDuplicateSnapshot` — full duplicate-sensitive snapshot |
| Outbox delivery vs business effect | `TestPrompt134OutboxDuplicateBusinessEffectOnce` — `delivery_attempt_count=2`, `business_effect_count=1` |
| Projection metadata optional | `TestPrompt134SixProjectionMandatoryMetadata` — mandatory fields must exist |
| Large-row families partial | `TestPrompt134LargeRowP1P2AllFamilies` — stock + ads + focus P2-only |
| P1 immutability wrong capture timing | `TestPrompt134PartiallySupportedP1BeforeAfter` — before/after P2 snapshot |
| Rollback Action state thin | four `TestPrompt134Rollback*ActionState` failpoints |
| Action groups unpopulated skip | `TestPrompt134ActionGroupImmutabilityPopulated` — requires `groups>0`, `members>0` |

## Production defects fixed (confirmed by stricter acceptance)

1. **Evidence bundle / Deep pack embedded JSON** — `evidence.MarshalBundleCanonical`, `deep.MarshalPayloadCanonical` wire paths; `NormalizeEvidenceDataBytes` on pack ingest (no byte-array `data` in AI-visible payloads).
2. **Continuity previous-action parameters dropped** — `previousFromContinuity` now copies `Parameters` for mirrored proposals / validation.
3. **Opposite price intent signatures** — `paramFingerprint` uses `ParametersWireFrom`; fake opposite-price pair + price menu for groups.
4. **Outbox idempotent business side effect** — aggregate-key registry (`TestOutboxProcessedAggregates`) — delivery may repeat, effect once.
5. **Continuity proposal validation** — `continuityProposalAllowed` merges base snapshot price hints.

No contract version bumps. No new migration.

## Mandatory acceptance matrix

| # | Gate | Exact scenario | Real PG | Test | Result |
|---|------|----------------|---------|------|--------|
| 1 | SUPERSEDED opposite intent | `SKU_ADJUST_PRICE` DECREASE→INCREASE | YES | `TestPrompt134SupersededOppositePriceIntentPostgres` | PASS |
| 2 | DISMISSED opposite intent | `SKU_ADJUST_PRICE` DECREASE→INCREASE | YES | `TestPrompt134DismissedOppositePriceIntentPostgres` | PASS |
| 3 | Continuity current value | P1=2, P2=40 stock | YES | `TestPrompt134ContinuityCurrentStockValues` | PASS |
| 4 | No stale current value | current ≠ stale 2 | YES | same | PASS |
| 5 | Over-budget provider count | `calls=0` | runtime | `TestPrompt134OverBudgetProviderCallsZero` | PASS |
| 6 | Concurrent loser residue | full snapshot | YES | `TestPrompt134ConcurrentPublishFullLoserResidue` | PASS |
| 7 | Seller lock independence | A/B (retained 13.3) | YES | `TestPrompt133SellerLockIndependence` | PASS |
| 8 | Unknown commit | post-COMMIT synthetic error | YES | `TestPrompt134UnknownCommitFullDuplicateSnapshot` | PASS |
| 9 | Unknown commit duplicates | full snapshot unchanged | YES | same | PASS |
| 10 | Outbox duplicate delivery | effect once | YES | `TestPrompt134OutboxDuplicateBusinessEffectOnce` | PASS |
| 11 | Projection metadata | six projections mandatory | YES | `TestPrompt134SixProjectionMandatoryMetadata` | PASS |
| 12 | Stock P1/P2 rows | P2 only | YES | `TestPrompt134LargeRowP1P2AllFamilies` | PASS |
| 13 | Ads P1/P2 rows | P2 only | YES | same | PASS |
| 14 | Focus P1/P2 rows | P2 only | YES | same | PASS |
| 15 | P1 before/after immutability | snapshot equal | YES | `TestPrompt134PartiallySupportedP1BeforeAfter` | PASS |
| 16 | Rollback Action state | all 4 failpoints | YES | `TestPrompt134Rollback*ActionState` | PASS |
| 17 | Action groups populated | count > 0 | YES | `TestPrompt134ActionGroupImmutabilityPopulated` | PASS |
| 18 | Action group UPDATE | rejected | YES | same | PASS |
| 19 | Action group DELETE | rejected | YES | same | PASS |
| 20 | Group member UPDATE | rejected | YES | same | PASS |
| 21 | Group member DELETE | rejected | YES | same | PASS |
| 22 | Full integration suite | command | YES | `go test -tags=integration -p 1 ./internal/analysis/...` | PASS |
| 23 | gofmt | empty | NO | `gofmt -l .` | PASS |
| 24 | go vet | pass | NO | `go vet ./...` | PASS |
| 25 | API build | pass | NO | `go build ./cmd/api` | PASS |
| 26 | worker build | pass | NO | `go build ./cmd/worker` | PASS |
| 27 | full go test | pass / EDR exception | NO | `go test -p 1 ./...` | PASS* |

\* **Gate 27 Windows note:** `internal/unifiedanalysis/prompt` may fail with `fork/exec … prompt.test.exe: Access is denied` (EDR). Equivalent renamed binary passes. All other packages PASS in session.

## Literal evidence (13.4 gates)

### Opposite intent (SUPERSEDED)

```text
old: SKU_ADJUST_PRICE / DECREASE / sig_old
new: SKU_ADJUST_PRICE / INCREASE / sig_new (sig_old ≠ sig_new)
old case: SUPERSEDED; new case: ACTIVE/OPEN
```

### Opposite intent (DISMISSED)

```text
old: SKU_ADJUST_PRICE / DECREASE — USER_DISMISSED suppression on old signature only
new: SKU_ADJUST_PRICE / INCREASE — ACTIVE/OPEN, not USER_DISMISSED suppressed
```

### Continuity stock

```text
P1 DB stock = 2
P2 DB stock = 40
AI-visible continuity-linked SKU_STOCK_HISTORY evidence: sellable_units=40 (not current=2)
```

### Over-budget Deep

```text
error = DEEP_PACK_TOO_LARGE
provider_calls = 0
no DEEP_AI accepted attempt / result artifact
```

### Outbox duplicate delivery

```text
test instrumentation: delivery_attempt_count = 2, business_effect_count = 1
production ProcessOutbox: mark processed_at only (no downstream consumer)
```

**Correction (Prompt 13.4A):** do not read the test-only aggregate counter as production downstream idempotency. See Outcome B in `PROMPT134A_TRACEABILITY.md`.

### Unknown-commit

Full `prompt134DuplicateSnapshot` equality across publications, components, findings, recommendations, projections, stock/ad/focus rows, action cases/proposals/assessments, groups/members, release events, outbox.

## Regression Prompt 9.7–13.3

| Suite | Result |
|-------|--------|
| Prompt 13 / 13.1 / 13.2 integration paths | PASS (within full `internal/analysis` integration run) |
| Prompt 13.3 matrix | PASS |
| Prompt 131 dismissed day29/31 | PASS (test hardened for price-hint signature match) |
| Scale / N+1 (13.3) | PASS |

## Remaining backend blockers

**NONE** for analysis Stages 1–19.

## Next step

Prompt 14 — Frontend / Dashboard + Actions migration (**NOT STARTED**).
