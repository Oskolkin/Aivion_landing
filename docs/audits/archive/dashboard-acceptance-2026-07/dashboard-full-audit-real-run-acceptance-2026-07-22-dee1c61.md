# Dashboard real-run acceptance §9 — 2026-07-22 (SHA dee1c61)

## Verdict

**acceptance failed** on `dee1c61eebb9780ab892af5ae048cf546ce0d183`.

Dashboard is **not** fully complete. Evidence saved; **this commit was not amended**.

---

## Fix commit (done)

| Field | Value |
| --- | --- |
| Full SHA | `dee1c61eebb9780ab892af5ae048cf546ce0d183` |
| Parent | `f6aac05acfb42e6c8f6d91a680ee87a4ffa909c2` (unchanged) |
| Subject | Fix advertising threshold percents via schema v10 threshold_claims |
| Prompt | `unified_analyst_v1_26` / `9b0fd12db06cb28acf3505d254b47b73303cdbd6c6b7c876142d58ff807e3308` |
| Schema | `aivion_unified_analysis_v10` / `sha256:8c2474226f0de69ec18709df7da9853fa5fb11e00522caf829e270bc3ecc673b` |

### Contract decision

Schema **v9** forbids unknown narrative fields (`additionalProperties: false`), so policy thresholds cannot be represented without a new immutable schema. Added structured `ad_efficiency.narrative.threshold_claims` in **v10** + prompt **v1_26**. Left **v9** / **v1_25** immutable.

### Target model shipped

* **Metric claims** — Pack facts (`ad_spend_share_of_total_store_revenue_pct`, `attributed_campaign_drr_pct`, `campaign_drr_pct`, …).
* **Threshold claims** — versioned registry (`high_drr_campaign_threshold` = `campaign_drr_pct` / `campaign` / `gt` / `80`; `low_stock_coverage_days_threshold` = `stock_coverage_days_7d` / `sku` / `lt` / `7`).

Unit fixtures (run 194 suite): PASS/FAIL cases covered. `go test ./internal/unifiedanalysis/...` **passed**. Frontend lint (warnings only), unit tests, production build **passed**.

Pre-existing unrelated failures observed in full `go test ./...`: `internal/config` env override when `DATABASE_URL` is set in the process; `cmd/devtools` browser fixture `no_actions` KeySKU cohort gap — not introduced by this commit.

---

## §9 run 195

| Field | Value |
| --- | --- |
| Status | `publication_blocked` |
| Error | `advertising_claim_percent_without_claim` for free-text **80.0%** (×2) |
| Prompt | `unified_analyst_v1_26` |
| Prompt hash | `9b0fd12…3308` |
| Schema (raw output) | `aivion_unified_analysis_v10` |
| Pack hash | `17e80bbe7eccb4f8…` (unchanged) |
| Actions | 3 |
| Cleanup | `completed` |
| Response ID | `resp_0f4a015e1a16de32006a608723e578819da3cd8953e585237b` |
| Publication | none; pointer still **74** |

### What improved vs run 194

| Issue | 194 | 195 |
| --- | --- | --- |
| Dual 26.2% + 53.3% metric claims | OK after f6aac05 | OK |
| Structured `threshold_claims` for policy 80 | absent | **present and correct** |
| Free-text «порог 80%» validation | blocked as share claim | **still blocked** (see root cause) |

### Narrative excerpt (`run-195-ad-narrative.json`)

```text
… По атрибутированным кампаниям ДРР составил 53,3% …, при этом 3 кампании превысили порог 80%. …
```

Structured claim emitted:

```json
{
  "policy_id": "high_drr_campaign_threshold",
  "metric_code": "campaign_drr_pct",
  "scope": "campaign",
  "operator": "gt",
  "value": 80
}
```

Artifacts: `docs/audits/real-run-2026-07-22-dee1c61/`.

---

## Root cause (do not fix in dee1c61)

Validator peels free-text percents into threshold handling only when `looksLikeThresholdPercentContext` matches a comparative **operator** token:

`>|≥|>=|<|≤|<=|выше|более|свыше|ниже|менее|меньше`

Run 195 prose uses **«превысили порог 80%»** — no listed operator, so 80% is treated as an ordinary share percent and fails `advertising_claim_percent_without_claim` even though a matching `threshold_claims` entry exists.

This is **not** a missing schema/prompt emission problem; the AI followed v1_26. The threshold-context detector is too narrow for Russian «превысил(и) порог N%» (and similar) phrasings.

---

## Gates not reached

Publication, Dashboard/Actions API dumps, repeated GET, Playwright, screenshots, rollback, restart/reconciliation — **not executed** (blocked on output validation).

---

## Next fix (separate commit — do not amend `dee1c61`)

Expand threshold-context recognition beyond symbols/`выше`/`ниже` to cover policy-threshold prose such as:

* `превысил(и) порог 80%`
* `порог 80%` / `выше порога 80%`
* equivalent campaign-DRR threshold wording

Keep requiring a matching structured `threshold_claim` from the registry (no regex-only allowlist for bare `ДРР > 80%`). Add a regression fixture from run 195 prose + claim.

---

## Completion checklist

| Criterion | Met? |
| --- | --- |
| Separate fix commit on top of f6aac05 | **yes** (`dee1c61…`) |
| Schema v10 + prompt v1_26 immutable | **yes** |
| Real Unified provider completed | yes |
| Output validation passed | **no** |
| Publication / Playwright / rollback | **n/a** |

**Final status: acceptance failed on SHA `dee1c61eebb9780ab892af5ae048cf546ce0d183` — stop here.**
