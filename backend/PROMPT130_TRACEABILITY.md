# PROMPT №13 TRACEABILITY

## Status

`PROMPT_13_STATUS = CLOSED` (see `backend/PROMPT134B_TRACEABILITY.md` for current superseding status).

**Historical:** this session did not execute the full CLOSED-criteria longitudinal/concurrency/rollback suite. **Current:** those gates are CLOSED as of Prompt 13.4A.

## Baseline

- branch: `codex/canonical-sales-hardening`
- SHA at start of Prompt 13 work: `267842e7c2907e1185a934156c46a71b982d8d7a`
- final SHA: uncommitted (no commit requested)
- Go: `go 1.25.6` / toolchain `go1.26.1`
- latest prior migration: `000089_prompt93_collection_provenance`
- new: `000090_action_cases`, `000091_analysis_publication`

## Deep Support Basis

Needed: **YES**

- `ActionSupportBasis` + canonical `B00N` keys on validate
- `PreviousSupportAssessment` coverage = previous REQUIRED_SUPPORT count
- versions: `deep_output_schema_v2`, `deep_validation_v2`, `deep_analysis_prompt_v2`, `deep_pack_schema_v2`
- Fake Deep emits ≥1 REQUIRED_SUPPORT and HOLDS assessments when previous_actions present

## Action identity

`action_identity_policy_v1` — canonical JSON SHA-256. System recurrence: `DISABLED_UNTIL_CALIBRATED` (no invented cooldown days). User DISMISS: 30 calendar days.

## Continuity

- `CONTINUITY_INITIAL_NO_BASE_V1` first run
- `CONTINUITY_FROM_PUBLICATION_V1` via `PrepareWithReader` + `LoadPreviousActions`
- `base_publication_id` captured on production `INSERT` from `seller_current_publications`
- Evidence `CONTINUITY_REQUIRED` expand-on-Stage-10: **not fully wired** (P2 still uses Discovery/Deep pack facts; limitation)

## Publish

Order: advisory xact lock → current pointer == run base → insert publication → components/findings/recs → apply plan → projections → upsert current → release events → outbox → run PUBLISHED → finalize PUBLISH + POST_PUBLISH READY → COMMIT.

EVALUATION: no publication rows.

## Tests run this session

- `go test` packages: deep, actionreconcile, continuity, projection, contracts, stages, runtime, actioncatalog — PASS
- `go build ./cmd/api ./cmd/worker` — PASS
- `go vet ./internal/analysis/...` — PASS
- integration (`TEST_DATABASE_URL` 55432, fake AI): `TestPrompt13FirstPublicationStages1to19`, `TestPrompt13SecondRunContinuity`, `TestPrompt13EvaluationCannotPublish`, `TestPrompt12DeepNonEmptyStages1to13` — PASS
- NOT RUN: concurrent two-run publish, DONE/DISMISS day29/day31 SQL fixtures, TX rollback injection, scale 700–2000 SKU N+1 counts, full `go test ./...`, MinIO smoke

## Remaining limitations

- system recurrence cooldown uncalibrated
- Stage 10 CONTINUITY_REQUIRED collection not complete
- Materiality silent-drop of continuity targets not fully fail-closed beyond Deep capacity 40
- HTTP/UI Action mutations: not in scope
- several CLOSED-criteria integration tests still to add
