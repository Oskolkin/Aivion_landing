# Dashboard real-run acceptance §9 — 2026-07-22 (fix SHA)

## Verdict

**acceptance failed** on fix commit `9aad781958c25eb74046db1b4d81d1b6d7eaaf56`.

Dashboard is **not** fully complete. Per policy: evidence recorded; **no further code fix in this commit**.

---

## Fix commit (prerequisite — done)

| Field | Value |
| --- | --- |
| Full SHA | `9aad781958c25eb74046db1b4d81d1b6d7eaaf56` |
| Parent | `cdafd4928d372650459d26560c32bf17ca4cf900` (unchanged) |
| Subject | Fix Unified acceptance blockers for prompt v1_25 |
| Prompt | `unified_analyst_v1_25` |
| Prompt hash | `b12f7a6b3745d9b9ece860bc04d77eae3f2f68140fdbdf8e0c055a453eee34fb` |
| Schema | `aivion_unified_analysis_v9` / `sha256:a4c3ede2825329de46f822edda057c22837d4345276b4bbddc4dad296f6a8717` |
| Health policy | `health_policy_v1` |
| Key SKU policy | `key_sku_cohort_v1` |
| Min actions (material) | `1` (was 5) |
| Min required actions | `1` (was 3) |

Regression: Go packages (dashboardfacts/home/healthpolicy/contract/prompt/outputvalidator/publication*) + frontend lint/test/build — **passed** before commit.

Evidence of prior failed acceptance on `cdafd49`: `docs/audits/dashboard-full-audit-real-run-acceptance-2026-07-22.md` + `docs/audits/real-run-2026-07-22/`.

---

## Preflight (this attempt)

| Check | Result |
| --- | --- |
| Working tree at commit | clean for code; untracked prior acceptance docs only |
| API / worker restarted on SHA | yes (`run-api.ps1` / `run-worker.ps1`) |
| Provider enabled | yes |
| Prompt profile | `analysis` |
| Prompt version on run | `unified_analyst_v1_25` |
| Schema hash in validation | pinned v9 |

---

## Pack

| Field | Value |
| --- | --- |
| Seller | `1` |
| Metric date | `2026-07-08` |
| Pack ID | `pack:1:2026-07-08:17e80bbe7eccb4f8` |
| Pack hash | `17e80bbe7eccb4f8cf7740e1d1ada796c22e4dd581b61c76874c38da036be854` |
| Archive SHA-256 | `2c4acd2799c6065f7538fbf52f2003f309284a26e3ecbc757eac1c389ad84eeb` |
| Technical validation | `valid: true` |

---

## Unified run 193

| Field | Value |
| --- | --- |
| Status | `publication_blocked` |
| Error | `output_contract_invalid` / `advertising_narrative_fact_mismatch: sentence cites conflicting percents 26.2% and 53.3% for store-revenue share` |
| Prompt | `unified_analyst_v1_25` / hash `b12f7a6b…34fb` |
| Model | `gpt-4.1` |
| Latency | 54512 ms |
| Tokens | in 13236 / out 9512 / total 22748 |
| CI calls | 1 |
| Actions emitted | **3** (min-5 floor no longer blocks) |
| Cleanup | `completed` |
| Response ID | `resp_0a90fba443e04a7e006a607db3b830819f9fba387b06827a98` |
| Container | `cntr_6a607db1a19c8191b3b60ffb0b588ae600d041ddfbeedfdc` |
| File | `cfile_6a607db34a888191af8765611595c60c` |
| Publication ID | none |
| Current pointer | still publication **74** (unchanged) |

Artifacts: `docs/audits/real-run-2026-07-22-v125/`.

### What improved vs runs 190/191

| Failure mode | 190/191 | 193 |
| --- | --- | --- |
| Bare «ДРР» for store share | blocked | **cleared** (uses «рекламная нагрузка» / store wording) |
| Missing `metric_claims` | blocked | **cleared** (both shares claimed) |
| Invented `sku:<metric>:…` fact IDs | blocked | **cleared** (no such errors) |
| `actions_count_below_minimum` (4 < 5) | blocked | **cleared** (3 actions accepted for count) |
| Conflicting store-share percents heuristic | — | **blocked** |

### Actual advertising text (grounded correctly)

From `run-193-ad-narrative.json`:

- Headline: store load **26,2%** + attributed campaign DRR **53,3%** in one sentence.
- Summary: same two metrics with correct denominators and matching `metric_claims`.
- Claims: `ad_spend_share_of_total_store_revenue_pct=26.23…`, `attributed_campaign_drr_pct=53.33…`.

So the model distinguished store share vs attributed DRR; the remaining gate is the **sentence-level conflict heuristic** treating both percentages as store-revenue share when they appear together.

---

## Gates not reached

Publication, API dumps, repeated GET, Playwright, screenshots, rollback, restart/reconciliation — **not executed**.

---

## Next fix (separate commit — do not amend `9aad781`)

Likely options (choose one, then full §9 re-run):

1. **Validator**: refine `advertising_narrative_fact_mismatch` so one store-share % and one attributed-DRR % in the same sentence are allowed when both have matching claims / distinct labels.
2. **Prompt v1_26**: require at most one share percentage per sentence (split store vs attributed into separate sentences) — softer workaround.

Recommended: (1) with a regression fixture from run 193 narrative, because the output is semantically correct.

---

## Completion checklist

| Criterion | Met? |
| --- | --- |
| Separate fix commit created | **yes** (`9aad781…`) |
| Real Unified provider completed | yes |
| Output validation passed | **no** |
| Publication completed | **no** |
| API / Playwright / rollback | **n/a** |

**Final status: acceptance failed on SHA `9aad781958c25eb74046db1b4d81d1b6d7eaaf56` — stop here.**
