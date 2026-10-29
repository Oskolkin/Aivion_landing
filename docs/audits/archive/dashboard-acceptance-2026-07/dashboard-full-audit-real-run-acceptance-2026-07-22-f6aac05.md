# Dashboard real-run acceptance §9 — 2026-07-22 (SHA f6aac05)

## Verdict

**acceptance failed** on `f6aac05acfb42e6c8f6d91a680ee87a4ffa909c2`.

Dashboard is **not** fully complete. Evidence saved; **no further code change in this commit**.

---

## Fix commit (done)

| Field | Value |
| --- | --- |
| Full SHA | `f6aac05acfb42e6c8f6d91a680ee87a4ffa909c2` |
| Parent | `9aad781958c25eb74046db1b4d81d1b6d7eaaf56` (unchanged) |
| Subject | Fix dual-metric advertising narrative percent matching |
| Prompt | still `unified_analyst_v1_25` / `b12f7a6b3745d9b9ece860bc04d77eae3f2f68140fdbdf8e0c055a453eee34fb` |
| Schema | `aivion_unified_analysis_v9` / `sha256:a4c3ede…8717` |

Change: claim-based 1:1 mapping for store share + attributed DRR in one sentence; run 193 dual-metric fixture PASS; conflicts / bare DRR / ambiguous labels still FAIL.

Regression (backend Wave 2 packages + FE lint/test/build): **passed** before commit.

---

## §9 run 194

| Field | Value |
| --- | --- |
| Status | `publication_blocked` |
| Error | `advertising_claim_percent_without_claim: free-text percentage 80.0% has no matching … claim` (×3) |
| Prompt | `unified_analyst_v1_25` |
| Pack hash | `17e80bbe7eccb4f8…` (unchanged) |
| Actions | 3 |
| Cleanup | `completed` |
| Response ID | `resp_0a6543ddcf1220a6006a608128f9cc819eb80371678154ad05` |
| Publication | none; pointer still **74** |

### Progress vs run 193

| Issue | 193 | 194 |
| --- | --- | --- |
| Dual 26.2% + 53.3% in one narrative | blocked (false positive) | **cleared** (claims present, dual metrics OK) |
| Ungrounded `80.0%` threshold prose | — | **blocked** |

### Narrative excerpt (`run-194-ad-narrative.json`)

- Store share **26,2%** + attributed DRR **53,3%** with matching claims — accepted by the new matcher.
- Extra free-text: «как минимум 2 дорогих (ДРР > **80%**)» — campaign threshold without a structured claim → hard fail under current percent-claim rule.

Artifacts: `docs/audits/real-run-2026-07-22-f6aac05/`.

---

## Gates not reached

Publication, API dumps, repeated GET, Playwright, screenshots, rollback, restart — **not executed**.

---

## Next fix (separate commit — do not amend `f6aac05`)

Likely options:

1. **Validator**: do not require share claims for comparative campaign thresholds (e.g. `ДРР > 80%`, `ДРР выше 80%`) when clearly campaign-local and not presented as cabinet store share / attributed aggregate.
2. **Prompt**: forbid unclaimed free-text percentages entirely (including thresholds) — force packing them into claims or qualitative wording.

Recommended: (1) with fixtures covering `ДРР > 80%` PASS vs unclaimed `80%` as store share FAIL.

---

## Completion checklist

| Criterion | Met? |
| --- | --- |
| Separate validator fix commit | **yes** (`f6aac05…`) |
| Real Unified provider completed | yes |
| Output validation passed | **no** |
| Publication / Playwright / rollback | **n/a** |

**Final status: acceptance failed on SHA `f6aac05acfb42e6c8f6d91a680ee87a4ffa909c2` — stop here.**
