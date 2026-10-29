# Dashboard final real-run acceptance — 2026-07-22 — SHA a18e153

## Verdict

```text
Dashboard full audit acceptance FAILED on runtime SHA a18e1538b0d506e3efbe6a141fc90b249e4ef9a1
```

| Field | Value |
| --- | --- |
| Exact gate | **Output validation** (`publication_blocked` / `output_contract_invalid`) |
| Exact error | `quick_action_required_reference_missing`: `top_action_ids must reference execution_tier=required actions` at `$.dashboard.top_action_ids[2]` |
| Evidence path | `docs/audits/real-run-2026-07-22-a18e153/` |
| Pointer state | **unchanged** — current publication remains **74** |
| Previous Dashboard preserved | **yes** — publication 74 still `published` and current |

Publication, API dumps, repeated GET fingerprints, Playwright, rollback, and restart/reconciliation were **not executed** after the blocking gate (per acceptance stop rules). No product code was changed on `a18e153`.

---

## Repository

```text
branch:        fix/provider-orchestration
runtime SHA:   a18e1538b0d506e3efbe6a141fc90b249e4ef9a1
parent SHA:    7ab439b55bfdc6f4179bf0f3719e70404a468ae6
working tree:  clean of product code changes
```

Pre-run: clean. Post-run untracked audit artifacts only:

- `docs/audits/real-run-2026-07-22-a18e153/`
- `docs/audits/dashboard-final-real-run-acceptance-2026-07-22-a18e153.md`
- local gitignored `.env.local` (copied from main worktree for `OPENAI_API_KEY`; not committed)

---

## Immutable contracts

| Contract | Version / hash | Result |
| --- | --- | --- |
| Schema | `aivion_unified_analysis_v10` / `sha256:8c2474226f0de69ec18709df7da9853fa5fb11e00522caf829e270bc3ecc673b` | PASS (unit + live request) |
| Prompt | `unified_analyst_v1_26` / `9b0fd12db06cb28acf3505d254b47b73303cdbd6c6b7c876142d58ff807e3308` | PASS (unit + live request) |
| Canonical LF | line-ending independence tests | PASS on Windows |

Note: validation artifact field `schema_version` still prints legacy label `aivion_unified_analysis_v1`, but live request/output/schema_hash are v10. Non-blocking reporting inconsistency.

---

## Automated preflight (Step 1)

### Backend

All PASS:

```text
go test ./internal/unifiedanalysis/prompt/...
go test ./internal/unifiedanalysis/contract/...
go test ./internal/unifiedanalysis/provider/...
go test ./internal/unifiedanalysis/outputvalidator/...
go test ./internal/unifiedanalysis/publication/...
go test ./internal/unifiedanalysis/publicationaudit/...
go test ./internal/unifiedanalysis/...
go test ./internal/jobs/...
```

### Frontend

| Check | Result |
| --- | --- |
| `npm run lint` | PASS (0 errors, 6 pre-existing warnings) |
| `npm test` | PASS |
| `npm run build` | PASS |

---

## Config (live worker / request)

| Setting | Value |
| --- | --- |
| model | `gpt-4.1` (response model id `gpt-4.1-2025-04-14`) |
| analysis max_output_tokens | **32768** (confirmed in request summary) |
| smoke max_output_tokens | **2000** (code default; unset in env) |
| max attempts | **2** (code default) |
| tool_choice | **auto** |
| poll timeout | 1800s default |
| provider enabled | true (worker + readiness) |
| publication / rollout | rollout true; auto-publish false; publish-as-current requested by devtool |
| prompt profile | analysis |
| prompt version | unified_analyst_v1_26 |
| schema version | aivion_unified_analysis_v10 |

Startup evidence:

- Worker: `unified analysis provider handlers registered` (`rollout_enabled=true`)
- API: `unified analysis runtime config` with rollout enabled
- Readiness: `Ready for unified provider smoke: YES`

Environment fix (no code commit): copied main-repo `.env.local` into worktree so `OPENAI_API_KEY` loaded via `scripts/dev/load-env.ps1`.

---

## Data Pack

Preflight `build-unified-analysis-pack` and live run 201:

| Field | Value |
| --- | --- |
| Pack ID | `pack:1:2026-07-08:17e80bbe7eccb4f8` |
| Pack hash | `17e80bbe7eccb4f8cf7740e1d1ada796c22e4dd581b61c76874c38da036be854` |
| archive_sha256 | `2c4acd2799c6065f7538fbf52f2003f309284a26e3ecbc757eac1c389ad84eeb` |
| compressed / uncompressed | 45002 / 270110 |
| technical validation | true |
| Determinism | **matched** historical hash for seller=1 / 2026-07-08 |

Row counts: account_summary=1, account_daily=30, sku_summary=94, sku_daily=1974, campaign_summary=10, campaign_daily=300, campaign_sku=110, product_enrichment=94.

Artifacts: `docs/audits/real-run-2026-07-22-a18e153/pack-preflight/`

---

## Live Unified run

| Field | Value |
| --- | --- |
| Command | `go run ./cmd/devtools run-unified-analysis --seller-id=1 --date-mode=fixed --metric-date=2026-07-08 --publish-as-current=true --execution=queue --wait=true --force=true --timeout=45m --cleanup=true` |
| Run ID | **201** |
| Started | 2026-07-22T11:43:06Z |
| Terminal status | `publication_blocked` |
| Pointer before | publication **74** / run 162 |
| Pointer after | publication **74** (unchanged) |

---

## Provider orchestration

### Attempt 1 (only attempt)

| Field | Value |
| --- | --- |
| attempt | 1 |
| response ID | `resp_0818f37096ed3b4e006a60acce0dd081a18d6e74fd422adc7b` |
| request ID | `req_7235fa068ef94f48a7bc064f222a5ba2` |
| retrieve request ID | `req_47482c35e9284c58bf751613a1958588` |
| status | **completed** |
| incomplete reason | (empty) |
| input / output / total tokens | 23883 / 12463 / 36346 |
| CI calls | 4 |
| assistant messages | 1 |
| output text bytes | 47631 |
| output item types | 4× `code_interpreter_call`, 1× `message` |
| latency | 90376 ms |
| refusal | false |
| raw response | stored; redacted copy at `artifact-unified_provider_response_raw.redacted.json` |

### Orchestration assertions

| Assertion | Result |
| --- | --- |
| completed + assistant message + structured output → no attempt 2 | **PASS** (`provider_attempt_count=1`) |
| max_output_tokens=32768 | PASS |
| tool_choice=auto | PASS |
| prompt/schema hashes match pinned | PASS |
| terminal attempt artifact before status change | PASS |
| cleanup only after logical terminal | PASS (`cleanup_status=completed`) |
| attempt count ≤ 2 | PASS |

Provider stage succeeded. Prior failures on SHA `67e5c5e` (runs 197/199/200) are not reproduced here.

---

## Validation

### Output validation — **FAIL (blocking)**

```text
valid=false
error_code=output_contract_invalid
errors:
  - code: quick_action_required_reference_missing
    path: $.dashboard.top_action_ids[2]
    message: top_action_ids must reference execution_tier=required actions
warnings (non-blocking):
  - dashboard_signal_duplicate @ $.dashboard.what_changed.groups[3].items[0]
  - dashboard_signal_duplicate @ $.dashboard.what_changed.groups[3].items[1]
```

### Root cause (product defect)

AI returned `material_actions_found` with 3 Actions:

| ID | execution_tier | type |
| --- | --- | --- |
| action_001 | required | stock_replenishment |
| action_002 | required | ad_high_drr_review |
| action_003 | **recommended** | no_sales_with_stock |

`dashboard.top_action_ids = [action_001, action_002, action_003]`

Index `[2]` violates the contract (and existing prompt rule): Top 3 may only reference `execution_tier=required`.

Evidence: `actions-tier-analysis.txt`, `artifact-unified_analysis_output_raw.json`, `output-validation.pretty.json`

### Bundle validation / publication

Not reached (blocked before publish). Correctly: current pointer not switched.

---

## Publication

| Field | Value |
| --- | --- |
| New publication | **none** |
| pointer before | 74 |
| pointer after | 74 |
| Dashboard snapshot | previous publication 74 remains |
| Actions run | previous (tied to publication 74) |
| publication audit of new run | N/A (not published) |

---

## API / Browser / Recovery

| Stage | Status |
| --- | --- |
| Dashboard/Actions API dumps for new publication | NOT RUN |
| Repeated GET semantic fingerprints | NOT RUN |
| Playwright smoke | NOT RUN |
| Full Playwright | NOT RUN |
| Screenshots | NOT RUN |
| Failure injection / rollback | NOT RUN |
| Restart / reconciliation | NOT RUN |

Baseline current publication inspect saved for continuity: `inspect-unified-publication-74.json`.

---

## What passed on this SHA (partial, not acceptance)

1. Unit/contract/hash suites green (prompt/schema pinned hashes).
2. Frontend lint/tests/build green.
3. Live Pack determinism unchanged.
4. Live OpenAI provider completed with structured output in one attempt under new orchestration settings.
5. Failed validation did **not** move current publication pointer; previous Dashboard remains available.
6. Provider cleanup completed after terminal logical result.

## What failed

1. Output contract: Top Action referenced a `recommended` Action.
2. Therefore no atomic publish, no immutable GET proof on a new publication, no Playwright acceptance, no rollback/restart proof on a newly published bundle.

---

## Proposed separate fix-commit (do NOT amend a18e153)

**Do not patch in place on acceptance SHA.** Suggested next commit on `fix/provider-orchestration` (or follow-up branch):

### Problem

Model occasionally places `execution_tier=recommended` IDs into `dashboard.top_action_ids`, despite prompt `unified_analyst_v1_26` already stating Top 3 must be required-only. Validator correctly blocks publication.

### Options (prefer both)

1. **Prompt bump** (immutable versioning): publish `unified_analyst_v1_27` with stronger explicit rule + negative example:
   - `top_action_ids` MUST contain only `required` IDs
   - put secondary Actions only in `recommended_action_ids` / Actions list
   - never promote `recommended` into Top 3 to “fill” three slots
2. **Deterministic sanitizer** (backend, before hard-fail or as allowed repair):
   - filter `top_action_ids` to Actions with `execution_tier=required`
   - optionally move demoted IDs into `recommended_action_ids`
   - keep validator as final gate
3. **Regression test**: fixture with Top3 containing a recommended ID → expect either sanitized pass or clear blocked code; live eval scenario optional.

### Acceptance re-run plan after fix

1. Land fix commit (no amend of `a18e153`).
2. Re-run one live Unified run for seller=1 / 2026-07-08 with `--publish-as-current=true`.
3. Continue from Step 6 through Step 13 (bundle → publish → API → GET fingerprints → full Playwright → rollback → restart).

---

## Evidence index

Directory: `docs/audits/real-run-2026-07-22-a18e153/`

| Artifact | Purpose |
| --- | --- |
| `pointer-before.txt` / `pointer-after.txt` | pointer 74 unchanged |
| `pack-preflight/` | Pack build + validate |
| `inspect-unified-provider-run-201.json` | run summary |
| `inspect-unified-analysis-output-201.txt` | validation embedded |
| `provider-request-summary.pretty.json` | tokens/tool_choice/hashes |
| `provider-terminal-attempt.pretty.json` | attempt diagnostics |
| `output-validation.pretty.json` | blocking error |
| `actions-tier-analysis.txt` | Top3 vs tiers |
| `artifact-unified_analysis_output_raw.json` | raw AI JSON |
| `artifact-unified_provider_response_raw.redacted.json` | redacted provider raw |
| `artifact-unified_dashboard_facts_v1.json` | backend facts snapshot |
| `inspect-unified-publication-74.json` | preserved previous current |

Runtime logs: `tmp/acceptance-logs/` (api/worker/run; local, not required in git).
