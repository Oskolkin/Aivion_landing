# Dashboard real-run acceptance — 2026-07-22 — SHA e48186d

## Verdict

```text
Dashboard full audit acceptance FAILED on runtime SHA e48186da85f4557172d1ee391bfb013ddd9e5a08
```

| Field | Value |
| --- | --- |
| Exact gate | **Output validation** |
| Exact error | `seller_pulse_ai_fact_mismatch` — fact_id `campaign:882000021528:DRR` / metric `campaign_drr_pct` not found in account_summary |
| Evidence path | `docs/audits/real-run-2026-07-22-e48186d/` |
| Pointer state | **unchanged** — current publication remains **74** |
| Previous Dashboard preserved | **yes** |

## Fix commit under test

```text
parent SHA:  0dc225f672ec7b27e30c6e6c5bbe1642e19a7891
fix SHA:     e48186da85f4557172d1ee391bfb013ddd9e5a08
branch:      fix/provider-orchestration
```

### What this fix solved (confirmed)

| Check | Result |
| --- | --- |
| Offline run 202 risk finalize + validate + ProjectDashboard | **PASS** |
| Live risk drivers | **canonical** — `high_drr_campaign`, `no_sales_with_stock` with separate `entity_ids` (no `high_drr_<id>` / `low_stock_<id>`) |
| Live `risk_driver_unknown_id` / `unified_dashboard_risk_driver_unknown_id` | **absent** |
| Prompt | `unified_analyst_v1_28` / `9edf9789afe1127ef94c3c78d83f20adc7f10105bbc174fb8166e717aa27e966` |
| Schema | `aivion_unified_analysis_v10` / `sha256:8c247422…ecc673b` (unchanged) |
| Validation report `schema_version` | **`aivion_unified_analysis_v10`** (reporting drift fixed) |
| Provider attempts | 1 (completed) |
| Pack hash | `17e80bbe…036be854` (deterministic) |
| Top actions | `top_action_count=2` |

### Live run

```text
run_id: 204
status: publication_blocked
error_code: output_contract_invalid
```

Output validation errors (only product blocker):

```text
seller_pulse_ai_fact_mismatch
path: $.dashboard.seller_pulse.narrative.important_facts[1].claims[0].fact_id
message: fact_id "campaign:882000021528:DRR" / metric "campaign_drr_pct" not found in account_summary
```

AI emitted a Pack-incompatible fact_id suffix `DRR` (likely should be a lowercase metric code such as `drr_pct`). This is a **new** product defect, unrelated to risk-driver registry composition.

## Diagnosis of prior run 202 defect (closed by this commit)

Dynamic IDs originated from **AI output** (`account_summary.risk_drivers[].id`), not backend health policy / publication builder.
Schema v10 already separates `id` (type code) from `entity_ids`; no schema bump required.
Backend now finalizes composites onto `health_policy_v1` codes before strict validation.

## Stop rule

New product defect discovered at output validation. **Not fixed in commit `e48186d`.** Full Playwright / GET fingerprints / rollback / publication pointer switch not run.

## Proposed next fix-commit (separate)

Address seller_pulse claim fact_id grounding:

- reject / normalize invented metric suffixes like `:DRR`;
- and/or prompt rule: copy fact_id metric segments only from Pack reference catalog;
- keep risk-driver finalization unchanged.

## Evidence

- `unified-run.log` — run 204 summary
- `inspect-output-204.txt` — validation detail + schema_version v10
- `inspect-provider-204.json`
- `inspect-publication-74.json` — preserved current
- `risk_drivers_raw_204.json` — canonical ids
- `pointer-and-status.txt`
- `output-validation-204.json`
