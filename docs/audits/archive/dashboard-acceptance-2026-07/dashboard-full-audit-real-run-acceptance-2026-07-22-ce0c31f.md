# Dashboard real-run acceptance §9 — 2026-07-22 (SHA ce0c31f)

## Verdict

**acceptance failed** on `ce0c31f354ee8d161ea433dad605b38a468743c7`.

Dashboard is **not** fully complete. Evidence saved; **this commit was not amended**.

---

## Fix commit (done)

| Field | Value |
| --- | --- |
| Full SHA | `ce0c31f354ee8d161ea433dad605b38a468743c7` |
| Parent | `dee1c61eebb9780ab892af5ae048cf546ce0d183` (unchanged) |
| Subject | Recognize Russian порог threshold prose in advertising validator |
| Schema / prompt | unchanged (`v10` / `v1_26`) |

Change: treat «превысил(и) порог», «выше/ниже порога», «(не) достиг(ла/ли) порога» as threshold *candidates*; still require an exact matching `threshold_claim`. Run 195 fixture + negatives for wrong value/operator/scope/missing claim.

Regression: `go test ./internal/unifiedanalysis/...` passed; FE lint (warnings only), unit tests, production build passed.

---

## §9 run 196

| Field | Value |
| --- | --- |
| Status | `publication_blocked` |
| Error | `advertising_claim_percent_without_claim` for **80.0%** (×2) |
| Also | `seller_pulse_ai_period_mismatch` on important_facts/key_points (secondary) |
| Prompt | `unified_analyst_v1_26` |
| Schema (raw) | `aivion_unified_analysis_v10` |
| Pack hash | `17e80bbe7eccb4f8…` (unchanged) |
| Actions | 2 |
| Cleanup | `completed` |
| Response ID | `resp_0fc3d85f540ea3c2006a6089cae9d081a0b6a6f408210cce3e` |
| Publication | none; pointer unchanged |

### Narrative (`run-196-ad-narrative.json`)

Headline:

```text
… но ДРР по 2 кампаниям превышает 80%
```

Summary (excerpt):

```text
… В 2 кампаниях ДРР выше 80%, …
```

Structured claim present and correct:

```json
{
  "policy_id": "high_drr_campaign_threshold",
  "metric_code": "campaign_drr_pct",
  "scope": "campaign",
  "operator": "gt",
  "value": 80
}
```

Artifacts: `docs/audits/real-run-2026-07-22-ce0c31f/`.

---

## Root cause (do not fix in ce0c31f)

`ce0c31f` covers **«… порог …»** phrasings from run 195. Run 196 used a different comparative form:

* **«превышает 80%»** (imperfective, **no** «порог») — not matched by the new phrase detector, so 80% stays on the share-claim path.
* Summary **«ДРР выше 80%»** *is* recognized as threshold context and grounds against `threshold_claims`. The duplicated `advertising_claim_percent_without_claim` comes from validating the same unpeeled headline 80% in both narrative-structure and free-text claim paths.

Unit replay of exact run 196 prose confirms: `looksLike(превышает 80%)=false`, `looksLike(ДРР выше 80%)=true`.

---

## Gates not reached

Publication, API dumps, repeated GET, Playwright, screenshots, rollback, restart/reconciliation — **not executed**.

---

## Next fix (separate commit — do not amend `ce0c31f`)

1. Extend threshold candidacy to imperfective/comparative verbs without requiring «порог», e.g. `превышает` / `превышают` / `превысил` when DRR/campaign cues are present (still require matching `threshold_claim`).
2. Optionally dedupe dual-path percent errors for the same free-text hit.
3. Fixture from run 196 headline+summary; keep negatives for invented thresholds.

Secondary: seller_pulse period_type on numeric key_points/important_facts (separate if it remains after advertising clears).

---

## Completion checklist

| Criterion | Met? |
| --- | --- |
| Separate fix commit on dee1c61 | **yes** (`ce0c31f…`) |
| Schema/prompt unchanged | **yes** |
| Real Unified provider completed | yes |
| Output validation passed | **no** |
| Publication / Playwright / rollback | **n/a** |

**Final status: acceptance failed on SHA `ce0c31f354ee8d161ea433dad605b38a468743c7` — stop here.**
