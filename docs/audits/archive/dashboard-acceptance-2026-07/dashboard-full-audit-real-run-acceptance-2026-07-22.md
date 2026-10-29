# Dashboard real-run acceptance §9 — 2026-07-22

## Verdict

**acceptance failed**

Dashboard **cannot** be called fully complete.

Pre-acceptance fixes and Wave 1+2 semantics are committed and unit/regression-tested. Real Unified publish on that SHA did **not** produce a new publication: provider/output validation blocked all attempts. Playwright, API artifact freeze, rollback, and restart gates were **not** executed (blocked on publication).

---

## 1. Commit under test

| Field | Value |
| --- | --- |
| Full SHA | `cdafd4928d372650459d26560c32bf17ca4cf900` |
| Subject | Stabilize unified dashboard publication semantics |
| Files | 78 (Wave 1 + Wave 2 + immutable prompts + sparse SKU Focus + audit docs) |
| Working tree at commit | clean (unrelated debug/chunk WIP stashed as `stash@{0}`) |
| Acceptance evidence tree | `docs/audits/real-run-2026-07-22/` (uncommitted evidence only; **not** folded into `cdafd49`) |

### Pinned versions / hashes (from commit)

| Artifact | Version | Hash |
| --- | --- | --- |
| Output schema | `aivion_unified_analysis_v9` | `sha256:a4c3ede2825329de46f822edda057c22837d4345276b4bbddc4dad296f6a8717` |
| Prompt (current) | `unified_analyst_v1_24` | `fd41711bfb17d783e3efce7e66e7003cb214538662990ae5ff1a2a061aa955d6` |
| Prompt (historical) | `unified_analyst_v1_22` | `67858013bb6e2f48eeb214338a891e977bd93508410d0a18d09a6e3aa0715180` |
| Prompt (intermediate) | `unified_analyst_v1_23` | `status = never_published` (no body; no fallback) |
| Health policy | `health_policy_v1` | (version stamp; score from full backend candidate registry) |
| Key SKU policy | `key_sku_cohort_v1` | 80% cumulative revenue cohort |

`TemplateByVersion(v1_22) != TemplateByVersion(v1_24)` and hashes differ (pinned in `prompt/template_test.go`).

---

## 2. Preflight (partial)

| Check | Result |
| --- | --- |
| Commit SHA fixed | yes — `cdafd49…` |
| Working tree clean before §9 | yes (then only evidence dirs appeared) |
| Unified provider enabled (worker) | yes (`scripts/dev/run-worker.ps1`) |
| Prompt profile | `analysis` |
| Prompt version | `unified_analyst_v1_24` |
| Schema version (provider output) | `aivion_unified_analysis_v9` |
| Accepted hashes | match pinned v9 / v1_24 (see validation artifacts) |
| Legacy fallback | not re-enabled for seller 1 |
| OpenAI | reachable (runs 190/191 completed provider stage) |
| API / worker / frontend | api PID active; worker PID active; frontend assumed on :3000 |
| Postgres / Redis / MinIO | Postgres `marketplace_ai_postgres`; Redis :6379; MinIO :19000 |
| Migrations | assumed applied (pipeline tables present) |

Note: PowerShell `2>&1` / NativeCommandError aborts `go run ./cmd/devtools` when it writes `date_resolution=` to stderr. Use `cmd /c "… > log 2>&1"` for reliable capture.

---

## 3. Pack

| Field | Value |
| --- | --- |
| Seller ID | `1` |
| Metric date | `2026-07-08` (fixed; stale historical, lag 14d) |
| Pack ID | `pack:1:2026-07-08:17e80bbe7eccb4f8` |
| Pack hash | `17e80bbe7eccb4f8cf7740e1d1ada796c22e4dd581b61c76874c38da036be854` |
| Pack archive SHA-256 | `2c4acd2799c6065f7538fbf52f2003f309284a26e3ecbc757eac1c389ad84eeb` |
| Storage key | `ai-analysis-packs/seller-1/2026-07-08/17e80bbe7eccb4f8…/analysis_pack.zip` |
| Row counts | `TotalRows=2613` (zip validation) |
| Technical validation | `valid: true` |
| Facts artifact hash (run 191) | `bfce4f6b531b0e63694c7369903b1e4e5fe7e628189d0155cf4c89c1e17e8400` (`unified_dashboard_facts_v1`) |
| Pack hash across runs 189/190/191 | **identical** (deterministic rebuild confirmed) |

Evidence: `docs/audits/real-run-2026-07-22/run-191-pack-generation-validation.json`, `runs-detail.txt`.

---

## 4. Unified runs

### Run 189 — provider failure

| Field | Value |
| --- | --- |
| Status | `failed` |
| Error | `provider_response_incomplete` / `provider response incomplete` |
| Model | `gpt-4.1` |
| Prompt | `unified_analyst_v1_24` |
| Provider status | `in_progress` at fail |
| Response ID | `resp_0613ceab97e28fd1006a607737a708819cb7204a487a1d4761` |
| Container | `cntr_6a607736212c8191a0c376ce657e001b0dc1fd7edf43bcc3` |
| File | `cfile_6a6077374cf48191930f02cf4e641c70` |
| Cleanup | `completed` |

Treated as provider/infra incompleteness, not a Dashboard semantics defect.

### Run 190 — output contract blocked (advertising claims)

| Field | Value |
| --- | --- |
| Status | `publication_blocked` |
| Error code | `output_contract_invalid` |
| Latency | 81210 ms |
| Tokens | in 14559 / out 10946 / total 25505 |
| CI calls | 1 |
| Cleanup | `completed` |
| Response ID | `resp_0d55a7af77c44a9c006a607834019481a1947e6f31cfe2f60b` |

Primary validation errors (Wave 2 claim validator working as designed):

- `advertising_bare_drr_for_store_share` — cabinet/account «ДРР» used as store-revenue share label
- `advertising_claim_percent_without_claim` — free-text `80.0%` without matching claim
- `advertising_narrative_fact_mismatch` — conflicting `26.2%` vs `80.0%`
- `non_russian_presentation_term` — `DRR` instead of `ДРР`

Evidence: `docs/audits/real-run-2026-07-22/run-190-output-validation.json`.

Provider request metadata confirms prompt version + hash:

```json
"prompt_version": "unified_analyst_v1_24",
"prompt_hash": "fd41711bfb17d783e3efce7e66e7003cb214538662990ae5ff1a2a061aa955d6"
```

### Run 191 — output contract blocked (fact grounding)

| Field | Value |
| --- | --- |
| Status | `publication_blocked` |
| Error code | `output_contract_invalid` |
| Latency | 103571 ms |
| Tokens | in 19986 / out 12845 / total 32831 |
| CI calls | 6 |
| Cleanup | `completed` |
| Response ID | `resp_0d99a39410723cc1006a6078a68d9881a28d7a035660d3de13` |
| Container | `cntr_6a6078a4afb481919a23845a090385f70e830869257781d3` |
| File | `cfile_6a6078a623188191b1f5a85aba110b54` |

Primary errors:

- Many `action_completion_baseline_missing` / `action_evidence_fact_unknown` — model invented IDs like `sku:stock_coverage_days_7d:801000100001:current_7d` (metric token in entity slot); not present in Pack facts registry
- `actions_count_below_minimum` — 4 actions vs required 5 for `material_actions_found`

Warnings also include duplicate seller-pulse / what-changed signal fingerprints and evidence scope inconsistency.

Provider raw preview starts with `"schema_version":"aivion_unified_analysis_v9"` (correct). Validation report field `schema_version` still prints `aivion_unified_analysis_v1` — reporting quirk only; schema hash is the pinned v9 hash.

Evidence: `run-191-output-validation.json`, `run-191-provider-request-summary.json`, `run-191-provider-response-summary.json`.

---

## 5. Output validation / publication / API / Playwright / rollback

| Gate | Status |
| --- | --- |
| Advertising metric claims | **failed** on run 190 (blocked before publish) |
| Entity / baseline fact grounding | **failed** on run 191 |
| Bundle validation | not reached |
| Publication | **not created** for 189–191 |
| Current pointer | unchanged: seller `1` → publication **`74`** (`metric_date=2026-07-08`, updated `2026-07-21`) |
| API dumps (`dashboard-api.json`, …) | **not collected** (no new publish) |
| Sparse SKU Focus live check | **not reached** |
| Repeated GET fingerprints | **not reached** |
| Playwright live | **not run** |
| Screenshots | **not saved** |
| Failure-injection / rollback / restart | **not run** |

---

## 6. Commands used

```text
# regression (pre-commit; all passed)
go test ./internal/dashboardfacts/...
go test ./internal/dashboardhome/...
go test ./internal/dashboardhome/healthpolicy/...
go test ./internal/unifiedanalysis/contract/...
go test ./internal/unifiedanalysis/prompt/...
go test ./internal/unifiedanalysis/outputvalidator/...
go test ./internal/unifiedanalysis/publication/bundlevalidator/...
go test ./internal/unifiedanalysis/publication/adapter/...
go test ./internal/unifiedanalysis/publication/...
go test ./internal/unifiedanalysis/publicationaudit/...
# frontend: npm run lint && npm test && npm run build

# commit
git commit → cdafd4928d372650459d26560c32bf17ca4cf900

# real runs (from backend/, env via scripts/dev/load-env.ps1)
go run ./cmd/devtools run-unified-analysis \
  --seller-id=1 --date-mode=fixed --metric-date=2026-07-08 \
  --publish-as-current=true --execution=queue --wait=true \
  --force=true --timeout=45m --cleanup=true
```

---

## 7. Remaining limitations / next steps

Per policy: **do not patch acceptance defects into `cdafd49`**. Required loop:

1. **acceptance failed** (this report)
2. Fix in a **new** commit, for example:
   - Prompt `v1_25` (immutable new body): stronger examples for Pack fact ID shape + store-share labeling / `metric_claims` discipline
   - Optionally tighten few-shot / negative examples for bare «ДРР» as store share
   - Investigate why model emits `sku:<metric>:<sku_id>:…` instead of registry IDs (pack dictionary / prompt clarity)
3. Re-run **full** §9 on the new SHA (pack → provider → validate → publish → API → Playwright → rollback → restart)
4. Only then mark Dashboard fully complete

Known process footguns:

- Do not start a second forced run while another active unified run exists (`uq_ai_pipeline_runs_one_active_unified_analysis`)
- Capture stderr via `cmd /c` on Windows PowerShell
- Worker must have `UNIFIED_ANALYSIS_PROVIDER_ENABLED` set in **its** process env

---

## 8. Completion checklist

| Criterion | Met? |
| --- | --- |
| Commit created | **yes** (`cdafd49…`) |
| Real Unified run completed (provider) | partial (190/191 completed; 189 incomplete) |
| Output validation passed | **no** |
| Publication completed | **no** |
| API payloads consistent | **n/a** |
| Playwright passed | **n/a** |
| Screenshots saved | **n/a** |
| Rollback passed | **n/a** |
| Restart / reconciliation passed | **n/a** |
| Working tree clean after successful acceptance | **no** (evidence dirs present; acceptance failed) |

**Final status: Dashboard not fully доработан — acceptance failed on SHA `cdafd4928d372650459d26560c32bf17ca4cf900`.**
