# Part 1 Report — Unified Analysis AI Contract v11

**Date:** 2026-08-06  
**Branch:** `feat/unified-analysis-v11-contract`  
**Baseline SHA:** `1f6a0670970abd01eaff6d6457a1e2e0d21293e8` (`main` / `origin/main` at audit start)

Part 2 is **not** started. Production current versions are unchanged.  
`part2_start_gate = BLOCKED_ON_PART1_MERGE` until PR #23 merges and the post-merge checklist completes.

---

## 1. Baseline SHA

`1f6a0670970abd01eaff6d6457a1e2e0d21293e8`

Documented in `docs/architecture/unified-analysis-v10-baseline.md`.

---

## 2. Studied current files (runtime truth)

- `contract/versions.go`, `schema.go`, `enums.go`, `output_v1.go`, `schema/unified_analysis_output_v10.schema.json`
- `prompt/template.go`, `versions.go`, `analyst_v1_31*.txt`, `hash.go`
- `outputvalidator/*` (SanitizeGrounding, SyncActionEntityCounts, RepairUnsupportedContentSteps, ReconcileStockActionCohorts, EnsureMaterialDomainActions, EnsureCategorySummaries, ScrubInternalFieldNames, FinalizeTopActionIDs, FinalizeDashboardRiskDrivers, FinalizeDashboardSectionFactReferences)
- `validate_output.go` Stage-4 order
- `thresholdpolicy/policy.go`, `pack/policy.go`, `pack/limitations.go`, `pack/dictionary.go`
- `publication/adapter`, `publication/policy.go`, `config.go`
- `docs/current/dashboard-field-ownership.md`, `docs/current/ai-pipeline.md`
- `actions/action_category_for_type.go`

---

## 3. Actual v10 problems (summary)

Heavy silent repair (delete stock Actions + regenerate; category score=50; scope promotion; target+10%); AI still owns facts that backend recalculates; weak Dashboard↔Actions linkage without Findings; prompt pressure for 8–12 Actions; raw numbers in narratives; mixed scope; AI Top Actions / risk distribution / health_score.

Full list: baseline doc §11–13.

---

## 4. New schema

- **ID:** `aivion_unified_analysis_v11`
- **File:** `backend/internal/unifiedanalysis/contract/schema/unified_analysis_output_v11.schema.json`
- **Hash:** `sha256:ea8ce2ed54699a832eadc42547e71053ce5f2048119df6e388b87a828cbecfb2` (`CanonicalSchemaHashV11`)
- **Go types:** `contract/output_v11.go`
- **Current remains:** `UnifiedAnalysisOutputSchemaVersionCurrent = V10`

---

## 5. New prompt

- **ID:** `unified_analyst_v1_32`
- **Files:** `prompt/analyst_v1_32.txt`, `analyst_v1_32_input.txt`
- **Hash (body):** `sha256:cc45e7c57ff80c168db5ff50f49658ea14769ddb8867ecafe118f22b2464dd11` (pinned in `prompt_v1_32_test.go`; via `HashForVersion` / `AnalystV1_32Hash`)
- **Current remains:** `CurrentAnalystVersion = unified_analyst_v1_31`
- Explicit ban on preferring fixed Action count bands; findings-first algorithm; claim/threshold placeholders only.

---

## 6. New policy

- **ID:** `analysis_policy_v2`
- **Package:** `backend/internal/unifiedanalysis/policy/v2/`
- **Hash:** `sha256:35f92ae0ba05cd4bdc08423472cd4a89b6d71d1dbd6b8bdaf9487456fa6d9063`
- 36 finding types; 13 Action types; production thresholds unchanged (DRR>80, coverage<7)
- **Not** wired into Pack builder / production runs

---

## 7. New contracts (complete list)

Analysis meta; analysis outcome; metric claim; threshold claim; finding (+ scope); account diagnosis (+ improvement lever); what changed (+ change item); category insights (+ item); funnel/stock/advertising insight (+ recommended check); seller pulse insight; SKU focus selection (+ item); action (+ plan step, completion union, object instruction); structured limitation.

---

## 8. Removed AI-owned fields (vs v10)

health_score / health_score_delta / severity / risk_drivers / metric_date on account; risk_distribution; top_action_ids / recommended_action_ids; Action.category; mixed scope; AI funnel stages / stock segments / ad factual summary / Seller Pulse KPIs; category_score / category severity/name generics; AI completion target_value / baseline inventing; free-string limitations; ok/partial/error statuses.

---

## 9. Fixtures

12 valid + 13 invalid under `backend/internal/unifiedanalysis/contract/testdata/v11/{valid,invalid}/`  
(with README + facts.json for valid; expect.json for invalid).

Invalid fixture gates (split):

| Gate | Status | Layer |
| --- | --- | --- |
| `invalid_structural_fixtures_gate` | PASS | `text` / `schema` |
| `invalid_reference_fixtures_gate` | PASS | `semantic` |
| `fact_registry_grounding_fixtures_gate` | DEFERRED_PART2 | `deferred_part2_fact_registry` (`unknown_fact_id`) |
| `entity_grounding_fixtures_gate` | DEFERRED_PART2 | `deferred_part2_entity` (`invalid_sku`) |

---

## 10. Tests

```text
cd backend
go test ./... -count=1
go vet ./...
```

**Local result (2026-08-06):**

- All Part 1 / UA packages: PASS
- `go vet ./...`: PASS
- Only local FAIL: `internal/jobs` (`TestRunAsynqUntilContext_gracefulShutdown`) — no Redis on `localhost:6379` (pre-existing env limitation; CI has Redis)
- Contaminated secret env cleared before clean suite; probe uses `OPENAI_API_KEY_FILE` only when `RUN_UA_V11_PROVIDER_SCHEMA_PROBE=1`

Production isolation tests (`v11_production_isolation_test.go`): PASS — current schema v10, prompt v1_31, v11 not in pack builder / provider default / publication accept path.

---

## 11. Provider schema result

| Item | Value |
| --- | --- |
| Gate | `provider_schema_compatibility_gate` |
| Result | **PASS** |
| Adapter | Responses API (`provider.NewOpenAIClient` + strict `json_schema`) |
| Model | `gpt-4.1` (`gpt-4.1-2025-04-14`) |
| Secret | runtime file via `OPENAI_API_KEY_FILE` (not printed) |
| Evidence | `backend/internal/unifiedanalysis/contract/testdata/v11/provider_schema_result.json` |

Confirmed: schema accepted at request; response `completed`; JSON parsed; decoded `UnifiedAnalysisOutputV11`; structural validation PASS. Artifact is anonymized (no secrets / no raw model content).

### Production model identity

| Item | Value |
| --- | --- |
| Gate | `production_model_identity_gate` |
| Result | **PASS** |
| Runtime source | `/opt/aivion/shared/runtime.env` on staging host `cv7780127.novalocal` |
| `OPENAI_MODEL` (api + worker containers) | `gpt-4.1` |
| `UNIFIED_ANALYSIS_MODEL` | unset/empty → UA config falls back to `OPENAI_MODEL` (`config.go`: `firstNonEmptyEnv("UNIFIED_ANALYSIS_MODEL", "OPENAI_MODEL")`) |
| Effective UA model | `gpt-4.1` |
| Probe model | `gpt-4.1` / response `gpt-4.1-2025-04-14` — **matches** production identity |

No separate mini/nano probe required (would not be equivalent to production-model acceptance).

---

## 12. Production current version unchanged

| Constant | Value |
| --- | --- |
| `UnifiedAnalysisOutputSchemaVersionCurrent` | `aivion_unified_analysis_v10` |
| `SchemaName()` / provider load | v10 |
| `CurrentAnalystVersion` | `unified_analyst_v1_31` |
| `PromptVersionAnalyst` | `unified_analyst_v1_31` |
| Publication accepted schema | still v10 path |

No seller 1 cutover; no publication pointer change; v10 repair logic untouched. v11 not wired into Pack builder or production provider default.

---

## 13. Documentation

- `docs/architecture/unified-analysis-v10-baseline.md`
- `docs/architecture/unified-analysis-contract-v11.md`
- `docs/architecture/unified-analysis-field-reference-v11.md`
- `docs/architecture/unified-analysis-policy-v2.md`
- `docs/architecture/unified-analysis-examples-v11.md`
- `docs/architecture/unified-analysis-v10-to-v11-mapping.md`

---

## 14. Repository / PR isolation

| Gate | Status | Evidence |
| --- | --- | --- |
| `part1_commit_scope_gate` | PASS | Branch commits contain only UA v11 Part 1 paths under `backend/internal/unifiedanalysis/**` and `docs/architecture/unified-analysis-*.md` |
| `part1_pr_diff_isolation_gate` | PASS | PR #23 file list = Part 1 only (`git diff --name-only origin/main...a77aa9f` has no non-Part-1 paths) |
| `working_tree_clean_gate` | FAIL | Local working tree still has unrelated dirty/untracked files (live-review, apex e2e, `tmp-*`, VPS scripts). Not committed; does not affect PR #23 |

Obsolete label `part1_repository_clean_gate` is **not** used.

---

## 15. CI on current PR HEAD `176bf7a`

| Item | Value |
| --- | --- |
| Current branch / PR HEAD | `176bf7a834a212434aef5528a7ae7aa6d3c13e41` |
| Includes gate-clarification commit | yes (`docs(unifiedanalysis): clarify Part 1 pre-merge acceptance gates`) |
| Historical green SHA (superseded) | `a77aa9fb8d9f510aa394b9b70ebabf0503230e75` — Backend CI https://github.com/Oskolkin/Aivion/actions/runs/31103994119 |

### Required jobs on `176bf7a` (`pull_request` event)

| Job | Result | Run |
| --- | --- | --- |
| backend | PASS | https://github.com/Oskolkin/Aivion/actions/runs/31107079319 |
| frontend | PASS | https://github.com/Oskolkin/Aivion/actions/runs/31107079352 |
| e2e | PASS | https://github.com/Oskolkin/Aivion/actions/runs/31107079352 (job e2e) |
| docker-build | PASS | https://github.com/Oskolkin/Aivion/actions/runs/31107080686 |
| gitleaks | PASS | https://github.com/Oskolkin/Aivion/actions/runs/31107079301 |

Backend PR run steps include: `gofmt`, `go vet ./...`, `go test ./...`, `go test -tags=integration -p 1 ./...`, build api/worker.

### Blocking duplicate (`push` event)

| Job | Result | Run |
| --- | --- | --- |
| e2e (push Frontend CI) | FAIL (not a test failure) | https://github.com/Oskolkin/Aivion/actions/runs/31107078210 |

Annotation: job did not start — GitHub billing / spending limit. Empty steps (~2s). This leaves PR `mergeStateStatus=UNSTABLE` despite all `pull_request` required jobs green.

Until the failed push `e2e` check is cleared (billing fix + successful rerun) or otherwise no longer blocking the rollup:

`latest_pr_head_ci_gate = PENDING`  
`part1_pr_ci_gate = PENDING`

---

## 16. Gates

| Gate | Status | Evidence |
| --- | --- | --- |
| repository_baseline_gate | PASS | baseline doc + SHA |
| v11_schema_gate | PASS | `ValidateSchemaStrict` / schema tests |
| v11_go_contract_gate | PASS | `output_v11.go` + decode fixtures |
| v11_field_reference_gate | PASS | field-reference doc |
| analysis_policy_v2_gate | PASS | `policy/v2` tests + hash pin |
| prompt_v1_32_gate | PASS | template registry + content asserts |
| prompt_hash_gate | PASS | pinned hash `cc45e7c5…` |
| schema_hash_gate | PASS | `CanonicalSchemaHashV11` |
| policy_hash_gate | PASS | `CanonicalPolicyHashV2` |
| fixtures_gate | PASS | 12 valid fixtures |
| invalid_structural_fixtures_gate | PASS | text/schema layers rejected |
| invalid_reference_fixtures_gate | PASS | semantic layers rejected |
| fact_registry_grounding_fixtures_gate | DEFERRED_PART2 | Fact Registry not in Part 1 |
| entity_grounding_fixtures_gate | DEFERRED_PART2 | entity grounding not in Part 1 |
| reference_integrity_gate | PASS | v11validate refs |
| raw_number_prohibition_gate | PASS | text rules + invalid fixture |
| finding_contract_gate | PASS | enums + semantic |
| dashboard_contract_gate | PASS | 8-block schema + fixtures |
| action_contract_gate | PASS | no category / no mixed |
| completion_contract_gate | PASS | anyOf without target_value |
| provider_schema_compatibility_gate | PASS | live Responses API probe artifact |
| production_model_identity_gate | PASS | runtime `OPENAI_MODEL=gpt-4.1` matches probe |
| production_version_unchanged_gate | PASS | Current* still v10/v1_31 |
| production_isolation_gate | PASS | isolation tests |
| current_publication_unchanged_gate | PASS | no publication wiring |
| full_backend_regression_gate | PASS | Backend CI on `176bf7a` PR run 31107079319 |
| part1_commit_scope_gate | PASS | Part 1 commits only |
| part1_pr_diff_isolation_gate | PASS | PR #23 Part 1 paths only |
| working_tree_clean_gate | FAIL | local unrelated dirty/untracked remain |
| latest_pr_head_ci_gate | **PENDING** | PR-event jobs green on `176bf7a`; push-event e2e FAIL (billing) → rollup UNSTABLE |
| part1_pr_ci_gate | **PENDING** | blocked on clean required-job rollup for current HEAD |
| part1_final_acceptance_gate | **PENDING** | blocked on `latest_pr_head_ci_gate` |
| part2_start_gate | **BLOCKED_ON_PART1_MERGE** | after merge: refresh main → verify SHA → contract/isolation on clean main → Part 2 branch → READY |

Obsolete labels: `part1_repository_clean_gate` is **not** used. `part2_start_gate` is **not** READY before merge.

**Do not merge on CI gate PASS until** push-event e2e failure is cleared and `latest_pr_head_ci_gate=PASS`. Fact Registry / Part 2 remain blocked.
