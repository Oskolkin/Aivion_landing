# PROMPT №13.2 TRACEABILITY

## Status

`PROMPT_13_2_STATUS = CLOSED` (see `backend/PROMPT134B_TRACEABILITY.md` for current superseding status).

`PROMPT_13_STATUS = CLOSED` · `PROMPT_13_4_STATUS = CLOSED` · `PROMPT_13_4A_STATUS = CLOSED`

## Baseline

- branch: `codex/canonical-sales-hardening`
- baseline SHA: `9072ca14ac5471b8c876ca3fb1514b83252ecfbc`
- Go: `go 1.25.6` / toolchain per `go.mod`
- migrations: `000090_action_cases`, `000091_analysis_publication`, `000092_action_support_revalidation` (**no new migration**)

## Deep Pack budget audit (`fitPayloadToBudget`)

| Item | Result |
|------|--------|
| Defect confirmed (13.1 `stripEvidence` / iterative truncation) | **YES** |
| Evidence `data` stripped on over-budget | **NO** (removed) |
| Over-budget behavior | `DEEP_PACK_TOO_LARGE` fail-closed; Stage 12 not called |
| Allowed compaction | Fact handles only in `fact_registry`; empty metadata omitted; secrets stripped |
| Safety margin calibration | `300` tokens (estimator slack at P2 continuity boundary ~86877 vs 86852) |
| Determinism | `TestDeepPackWithinBudgetDeterministic` PASS |

## Integration matrix (section 50)

| # | Scenario | Result |
|---|----------|--------|
| 1 | Factual Evidence preserved | PASS (unit) |
| 2 | Over-budget gate | PASS (unit) |
| 3 | Continuity factual visibility | PASS (131 + unit) |
| 4 | PARTIALLY_SUPPORTED Postgres | PASS |
| 5 | NOT_SUPPORTED Postgres | PASS |
| 6 | UNVERIFIABLE Postgres | PASS |
| 7 | SUPERSEDED Postgres | PASS (assessment + system state) |
| 8 | DONE → immediate recurrence | PASS |
| 9 | DISMISSED opposite intent | PASS |
| 10 | Concurrent `PublishAtomic` | PASS |
| 11 | Rollback after publication insert | PASS |
| 12 | Rollback after action mutations | PASS |
| 13 | Rollback after projections | PASS |
| 14 | Rollback before current pointer | PASS |
| 15 | Publish idempotent retry | PASS |
| 16 | Publication SQL immutability | PASS (core tables) |
| 17 | Mutable user Action state | PASS (DONE overlay) |
| 18 | Publication hash stable after DONE | PASS |
| 19 | Outbox atomicity on rollback | PASS |
| 20 | POST_PUBLISH outbox retry | PASS |
| 21 | Duplicate outbox processing | PASS |
| 22 | Single publication consistency | PASS (131) |
| 23 | 2000 SKU projection scale | PASS |
| 24 | No N+1 (`ListSKUStock`) | PASS |

## Key code changes

- `deep/pack_builder.go`: fail-closed budget; compact evidence projection; price from evidence into snapshot
- `deep/budgets.go`: safety margin `300`
- `postgres/publish.go`: idempotent retry without `LEASE_LOST`
- `actioncatalog/menu.go`: `PriceHintFromMenu` for price intent
- `actionreconcile/planner.go`: menu price hint for `IntentSignature`
- `prompt132_integration_test.go`: full acceptance suite

## Tests executed

```bash
go test ./internal/analysis/deep/...
go test ./internal/analysis/evidence/... ./internal/analysis/actionreconcile/...
go test -tags=integration -p 1 ./internal/analysis/ -run TestPrompt132
go test -tags=integration -p 1 ./internal/analysis/ -run TestPrompt131
go vet ./...
go build ./cmd/api ./cmd/worker
```

All above: **PASS** (integration requires `TEST_DATABASE_URL` + Postgres).

## Versions

- `deep_pack_budget_v1` (safety margin value changed; version unchanged — estimator calibration only)
- No `deep_pack_schema` bump (factual fields preserved; fact handles deduplicated to registry)

## Next step

Frontend / Dashboard + Actions read migration — **NOT IMPLEMENTED**
