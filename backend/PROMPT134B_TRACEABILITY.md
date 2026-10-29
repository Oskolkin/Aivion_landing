# PROMPT №13.4B TRACEABILITY

## Status

`PROMPT_13_4B_STATUS = CLOSED`

`PROMPT_13_STATUS = CLOSED` · `PROMPT_13_1_STATUS = CLOSED` · `PROMPT_13_2_STATUS = CLOSED` · `PROMPT_13_3_STATUS = CLOSED` · `PROMPT_13_4_STATUS = CLOSED` · `PROMPT_13_4A_STATUS = CLOSED`

Backend analysis pipeline **Stages 1–19 = CLOSED**.

Remaining backend blockers before Prompt 14: **NONE**

This is the last Prompt 13.x. No Prompt 13.4C / 13.5.

## Baseline (session)

| Item | Value |
|------|-------|
| branch | `codex/canonical-sales-hardening` |
| SHA | `005e18c863d352673950692e9ac906d13eb4264b` |
| Go | `go1.26.1` / module `go 1.25.6` |
| working tree | uncommitted Prompt 13.4B acceptance (no commit requested) |
| latest migration | `000092_action_support_revalidation` |
| NEW_MIGRATION | **NO** |
| gofmt | `gofmt -l .` empty |

## Five remaining gaps (after 13.4A)

| Gap | 13.4A actual | 13.4B proof |
|-----|--------------|-------------|
| Concurrent loser | seller counts + run residue; not full ID-set / overlay digest | `TestPrompt134BConcurrentFullLoserState` |
| Continuity Fact Handle/FactRef | P1/P2 handles + value 40; weak evidence/ref binding | `TestPrompt134BContinuityGroundingFactRef` |
| P1 immutable hash | subset columns (ordinal/statement/title) | full semantic columns SHA-256 |
| Large-row sentinels | unused `P1_*` string args; only `publication_id` | real `sellable_units` 111/222 and campaign `budget_amount` 333/444 |
| Overview cutoff | `AnalysisCutoffAt != ""` | literal `==` official projection timestamp |

## Production change (minimal)

`projection.buildRows` now persists legitimate Factual API fields already returned by `ListSKUStock` / `ListCampaigns`:

- Stock / SKU Focus: `sellable_units` (+ Focus `display_order`)
- Advertising: `budget_amount` when Budget is `KNOWN`

No schema migration. No contract version bump (additive row JSON).

## Fact Handle contract

**Lineage/fingerprint specific, not a stable semantic ID across revisions.**

- Discovery `f_*` handles are reused only when the lineage fingerprint (entity + semantic path + FactRefs + formula) matches.
- Evidence materialization assigns a new `ef_*` handle when the fingerprint differs (typical when `ResultHash` / FactRef changes with current facts).

P1 vs P2 stock current grounding uses distinct evidence IDs and distinct handles.

## Mandatory gates

| # | Gate | Required proof | Real PG | Test | Result |
|---|---|---|---|---|---|
| 1 | Concurrent loser state | loser contribution 0 + winner-only overlay mutation | YES | `TestPrompt134BConcurrentFullLoserState` | PASS |
| 2 | Continuity Fact Handle/FactRef | non-empty current_fact_refs_json + P2 grounding | YES | `TestPrompt134BContinuityGroundingFactRef` | PASS |
| 3 | P1 immutable content hash | full semantic snapshot equal | YES | `TestPrompt134BPartiallySupportedP1FullContentHash` | PASS |
| 4 | Stock/Ads/Focus sentinels | P2 present, P1 absent, mix=0 | YES | `TestPrompt134BLargeRowSentinelCurrentRead` | PASS |
| 5 | Overview cutoff literal | Overview == P2 cutoff serialization | YES | `TestPrompt134BOverviewCutoffLiteral` | PASS |
| 6 | Seller lock independence | A and B concurrent | YES | `TestPrompt134BSellerLockIndependence` | PASS |
| 7 | gofmt | empty | NO | `gofmt -l .` | PASS |
| 8 | go vet | PASS | NO | `go vet ./...` | PASS |
| 9 | API build | PASS | NO | `go build ./cmd/api` | PASS |
| 10 | worker build | PASS | NO | `go build ./cmd/worker` | PASS |
| 11 | full go test | PASS / EDR-only | NO | `go test -p 1 ./...` | PASS* |
| 12 | full analysis integration | PASS | YES | `go test -tags=integration -p 1 ./internal/analysis/...` | PASS |

## Concurrent evidence (sample run)

```text
winner run/id = 2448 / publication 1478
loser run/id  = 2447 / publications 0

before seller snapshot hash = f95447550eb8e43760bed0b7419628676eeb0e2e0d284463af4b63add2170315
after snapshot hash         = 7ea91c29763248c41e102a5f71a9abc9fa0399db2c4164d398750034e963fb16

winner-owned deltas:
  publications 1→2
  outbox 1→2
  current pointer 1477→1478

loser:
  publications = 0
  pre-existing case_state mutations not winner-owned = 0
  pre-existing signature_state mutations not winner-owned = 0
```

Lock path remains `pg_advisory_xact_lock` (seller-scoped). No global Go mutex.

\* `internal/unifiedanalysis/prompt`: `prompt.test.exe Access is denied`. Equivalent `go test -c -o prompt_pkg_tests.exe` then run binary → **PASS**.

## Continuity evidence (sample run)

```text
Fact Handle contract: lineage/fingerprint specific
  (discovery f_* reused iff fingerprint matches; else evidence ef_*)

P1 value = 2
P1 evidence = ev_001
P1 handle = ef_0001

P2 value = 40
P2 evidence = ev_002
P2 handle = ef_0019
P2 FactRef Kind = SKU_STOCK (sealed ResultHash changed vs P1)

current reassessment grounding = P2 pack handles (all support rows: current_fact_refs_json MUST NOT be empty; each row grounded to P2 handle/FactRef; P1-only stale = NONE)
stale current value 2 = NONE
stale grounding = NONE
```

## Immutable P1 hash (sample run)

```text
included tables:
  analysis_publications (identity + content hashes + cutoff + universe + published_at)
  publication_components, findings, recommendations, projections
  publication_stock_sku_rows, publication_ad_campaign_rows, publication_sku_focus_rows
  publication_action_groups, publication_action_group_members
  action_case_proposals, action_proposal_supports

excluded:
  action_case_state, action_signature_state, action_user_events, action_system_events
  reason: mutable Action overlay (contract allows mutation after P2)

  surrogate serial ids on findings/recommendations/components (ordered by business keys)
  reason: not publication business content

H_before = d06e40d5a7ee1fbf980d2e9623bc40bd39949124ffd79e68a6b93ee6fa634ab8
H_after  = d06e40d5a7ee1fbf980d2e9623bc40bd39949124ffd79e68a6b93ee6fa634ab8
equal = YES
```

## Large-row current-read (sample run)

Official path: `seller_current_publications` → `GetCurrentPublication` → `SELECT … WHERE publication_id = current.id`.

HTTP Dashboard readers remain Prompt 14.

```text
Stock:       P1 sentinel sellable_units=111  P2=222  current=1486  P2 found=YES  P1 found=NO  mix=0  row_count=12
Advertising: P1 sentinel budget_amount=333   P2=444  current=1486  P2 found=YES  P1 found=NO  mix=0  row_count=2
Focus:       P1 sentinel sellable_units=111  P2=222  current=1486  P2 found=YES  P1 found=NO  mix=0  row_count=12
```

## Overview cutoff (sample run)

```text
P2 analysis_cutoff_at (official Overview serialization) = 2026-08-25T12:00:00Z
Overview.analysis_cutoff_at = 2026-08-25T12:00:00Z
equal = YES
P2 publication_id = 1488
seller = 1964
analysis_date = 2026-08-24
universe = 9ab24296adadb5a9234c3197eeada5b4563efc42db56ca5bd0c3fcc4c5d6f268
```

## Test counts (programmatic)

```text
TestPrompt134*  (excluding 134A/134B names) = 15
TestPrompt134A* = 9
TestPrompt134B* = 6
```

Existing Prompt 134 / 134A tests retained.

## Versions

Unchanged.

## Migration

**NO** — row JSON additive fields only.

## Next step

Prompt 14 — Frontend / Dashboard + Actions migration — **NOT STARTED**
