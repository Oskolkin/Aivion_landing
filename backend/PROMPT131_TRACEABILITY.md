# PROMPT №13.1 TRACEABILITY

## Status

`PROMPT_13_1_STATUS = CLOSED` (see `backend/PROMPT134B_TRACEABILITY.md` for current superseding status).

**Historical (Prompt 13.1 session):** several CLOSED-matrix items were NOT RUN / SKIPPED at that time. Those gates were later proven in Prompts 13.2–13.4A. The line `Prompt 13.1 = CODE_COMPLETE_VERIFY_PENDING` below is **historical** and is not current project status.

Core gap **CONTINUITY_REQUIRED Evidence on Stage 10** is wired and covered by integration tests. P1→P2 continuity, no-discovery-threads continuity, FULLY_SUPPORTED second proposal, DISMISSED day-29/31 planner gates, single-publication consistency, and Prompt №13 regressions pass. Several mandatory CLOSED-matrix items remain **NOT RUN** or **SKIPPED** (concurrent Stage-18 publish, rollback failpoints, five-outcome Postgres matrix, DONE Postgres, scale/N+1 gate, immutability SQL, outbox idempotency, publish idempotent retry).

## Baseline

- branch: `codex/canonical-sales-hardening`
- baseline SHA: `9dc48aa8440ea475583c4ed49fa60dcc6cedcdc7`
- working tree: uncommitted Prompt 13 + 13.1 changes (no commit requested)
- Go: `go 1.25.6` / toolchain `go1.26.1`
- migrations: `000090_action_cases`, `000091_analysis_publication`, **new** `000092_action_support_revalidation`

## Gaps audit (Prompt 13 report)

| Gap | Confirmed at start | Resolution |
|-----|-------------------|------------|
| A. CONTINUITY_REQUIRED not wired | YES | `evidence/continuity.go`, expand union, consumer refs, bundle relations |
| B. Five outcomes Postgres | YES | Only FULLY_SUPPORTED end-to-end in 13.1; others NOT RUN |
| C. DONE/DISMISSED Postgres | PARTIAL | DISMISSED day-29/31 via planner unit path; DONE Postgres NOT RUN |
| D. Concurrent publish / rollback | YES | SKIPPED in integration (one active run/seller); failpoint hook added |
| E. Single-publication consistency | PARTIAL | All six projection keys asserted on P2 rows |

## CONTINUITY_REQUIRED Evidence

- **Support revalidation contract**: `SupportRevalidationSpec` on Deep `ActionSupportBasis`, continuity `PackedSupport`, persisted `revalidation_json` (`000092`).
- **Request generation**: `CollectContinuityRequests` reads previous `REQUIRED_SUPPORT` revalidation specs (no action-type switch).
- **Stage 10 union**: Discovery requests ∪ continuity requests → `NormalizeAllRequests` dedup by type/subject/related/universe.
- **Consumer refs**: `EvidenceConsumerRef` distinguishes `DISCOVERY_REQUESTED` vs `CONTINUITY_REQUIRED`.
- **Bundle relations**: `ContinuitySupportEvidenceRelation` + `continuity_support_evidence_links` in Deep Pack.
- **Coverage gate**: `ValidateContinuityCoverage` hard-fails when relation count ≠ previous required-support count.

Versions: `evidence_request_normalization_v2`, `evidence_request_set_schema_v2`, `evidence_bundle_schema_v2`, `action_continuity_policy_v2`.

## Deep continuity retrofit

- `deep_output_schema_v3`, `deep_validation_v3`, `deep_analysis_prompt_v3`, schema name `aivion_deep_analysis_output_v3`.
- `PreviousSupportAssessment` required per previous `REQUIRED_SUPPORT`.
- Deep Pack: filter fact registry to bundle/thread/continuity-needed handles; omit evidence `data` blobs (facts in registry); `fitPayloadToBudget` when estimate exceeds budget.
- Fake Deep: no `ActionProposals` on `NO_MEANINGFUL_THREADS`; continuity mirror proposal for `FULLY_SUPPORTED` second runs.

## Publish hardening (this session)

- Persist `action_case_assessments` + `action_support_assessments` on publish from `PreviousOutcomes`.
- `TestPublishFailpoint` hook in `postgres/publish_failpoint.go` (integration stage path still skips rollback assertion).

## Integration matrix (mandatory)

| # | Scenario | Result |
|---|----------|--------|
| 1 | P1→P2 CONTINUITY_REQUIRED | PASS |
| 2 | Continuity + NO_MEANINGFUL_THREADS | PASS |
| 3 | FULLY_SUPPORTED | PASS |
| 4 | PARTIALLY_SUPPORTED | NOT RUN |
| 5 | NOT_SUPPORTED | NOT RUN |
| 6 | UNVERIFIABLE | NOT RUN |
| 7 | SUPERSEDED | NOT RUN |
| 8 | DONE → new occurrence | NOT RUN |
| 9 | DISMISSED day 29 | PASS (planner) |
| 10 | DISMISSED day 31 | PASS (planner) |
| 11 | Opposite intent bypass | NOT RUN |
| 12 | Concurrent same-seller publish | SKIPPED |
| 13 | Stale run no writes | PASS (Prompt 13) |
| 14–16 | Rollback failpoints | SKIPPED |
| 17 | Publish idempotent retry | NOT RUN |
| 18 | Single publication consistency | PASS |
| 19 | Publication DB immutability | NOT RUN |
| 20 | User state ≠ publication hash | NOT RUN |
| 21 | Outbox atomicity/retry | NOT RUN |
| 22 | Evaluation cannot publish | PASS (Prompt 13) |
| 23 | Continuity capacity fail-closed | NOT RUN |
| 24 | Scale projection N+1 | NOT RUN |

## Tests executed

```bash
go test ./internal/analysis/deep/... ./internal/analysis/evidence/... \
  ./internal/analysis/actionreconcile/... ./internal/analysis/continuity/... \
  ./internal/analysis/projection/... ./internal/analysis/contracts/... \
  ./internal/analysis/stages/...   # PASS

go vet ./internal/analysis/...
go build ./cmd/api ./cmd/worker   # PASS

go test -tags=integration -p 1 ./internal/analysis/ -run 'TestPrompt131'  # PASS (2 SKIP)
go test -tags=integration -p 1 ./internal/analysis/ -run 'TestPrompt13'   # PASS
```

## Remaining blockers before frontend

1. Postgres integration for PARTIALLY_SUPPORTED / NOT_SUPPORTED / UNVERIFIABLE / SUPERSEDED.
2. DONE user-event Postgres + immediate second occurrence gate.
3. `PublishAtomic`-level concurrent winner + rollback DB assertions (not only Stage-18 executor).
4. Publish idempotent retry after unknown commit.
5. Publication immutability triggers SQL suite.
6. Outbox atomicity + duplicate processing.
7. Scale fixture (~700–2000 SKU) projection timing / N+1 static gate.

## Backend pipeline state

Stages **1–19 implemented** and regression-stable for Prompt 9.7–13 paths. **Historical:** Prompt **13.1 = CODE_COMPLETE_VERIFY_PENDING** at the end of the 13.1 session (not CLOSED until later matrix work). **Current:** CLOSED via Prompt 13.4A.

## Next step

Frontend / Dashboard + Actions read migration — **NOT IMPLEMENTED**.
