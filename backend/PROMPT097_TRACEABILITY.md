# Prompt №9.7 Traceability Matrix

Factual record of the already-closed Prompt №9.7 edge-case closure. This file was written during Prompt №9.8 to close traceability drift (`PROMPT094`–`PROMPT096` existed; 9.7 regressions already lived in code).

## Baseline (as of Prompt №9.8 start)

| Field | Value |
|-------|-------|
| Branch | `codex/canonical-sales-hardening` |
| HEAD at 9.8 start | `f609db3212253a16ff1557f850cd4246c0a42664` |
| WORKTREE at 9.8 start | clean |
| LATEST_MIGRATION | `000089_prompt93_collection_provenance` |
| Go module | `go 1.25.6` |

Prompt №9.7 itself was committed before this SHA. Exact 9.7 commit message/SHA mapping and live SQL counts are **not independently reconstructed from this repository snapshot** beyond: HEAD at 9.8 start is `f609db3`, and the 9.7 regression files listed below exist in that tree.

## Intent

Close remaining deterministic semantic defects after Prompt №9.6 before Discovery. Stage 7 remained NOT_IMPLEMENTED.

## Code present in the 9.8 baseline tree

| Area | Evidence in tree |
|------|------------------|
| Materiality ALL/ANY | `backend/internal/analysis/materiality/ranking.go` uses `CapabilitiesAllowLane` |
| Detail 15/40/15 | `backend/internal/analysis/query_counter_test.go` |
| Advertising periods fail-closed | `backend/internal/analysis/prediscovery_v4.go` |
| Unknown ad counts ≠ zero | `CountAccumulator` / `CampaignCountWithSpend` in landscape advertising path |
| Campaign stock bounded vs assigned | `backend/internal/analysis/landscape/campaign_derive.go` |
| DomainWindows sort | Domain then FactFamily |
| Category search exact quantiles | `backend/internal/factual/category_v4.go` (`big.Rat`, no float64 quantile) |
| Funnel LIMITED/UNKNOWN ≠ READY_EMPTY | `backend/internal/analysis/landscape/funnel_state.go` |

## Regression files

- `backend/internal/analysis/prompt97_regression_test.go`
- `backend/internal/analysis/materiality/prompt97_capability_test.go`
- `backend/internal/analysis/landscape/prompt97_semantics_test.go`
- `backend/internal/analysis/landscape/prompt97_static_gate_test.go`

## Session measurements (Prompt №9.7 chat evidence, not re-run here)

| Gate | Claimed in 9.7 session | Reconstructed now |
|------|------------------------|-------------------|
| Core 500 SQL | 69 | not independently reconstructed from repository snapshot |
| Core 2000 SQL | 100 | not independently reconstructed from repository snapshot |
| Detail 15/40/15 SQL | 27 | not independently reconstructed from repository snapshot |
| Stage 7 | NOT_EXECUTED | confirmed by `ImplementedInCurrentBuild=false` in `stages/registry.go` |
| `RACE` | NOT_RUN (Windows/CGO) | not re-run |

## Architecture

Normative authority remained `docs/architecture/v1` + final Physical Implementation Plan. No Discovery implementation.
