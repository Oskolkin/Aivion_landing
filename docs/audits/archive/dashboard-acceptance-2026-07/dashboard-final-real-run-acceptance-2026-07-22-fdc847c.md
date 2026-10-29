# Dashboard real-run acceptance — 2026-07-22 — SHA fdc847c

## Verdict

```text
Dashboard full audit acceptance FAILED on runtime SHA fdc847c98b09794b07749ecb66bb9575b91ed0ca
```

| Field | Value |
| --- | --- |
| Exact gate | **Bundle validation / publication** |
| Exact error | `advertising_total_revenue_share_mismatch` — narrative store-revenue share **53.3%** conflicts with facts **~26.6%** (`ad_efficiency.narrative.summary`) |
| Evidence path | `docs/audits/real-run-2026-07-22-fdc847c/` |
| Pointer state | **unchanged** — current publication remains **74** |
| Previous Dashboard preserved | **yes** |

## Fix commit under test

```text
parent SHA:  e48186da85f4557172d1ee391bfb013ddd9e5a08
fix SHA:     fdc847c98b09794b07749ecb66bb9575b91ed0ca
branch:      fix/provider-orchestration
```

| Contract | Value |
| --- | --- |
| Schema | `aivion_unified_analysis_v10` / `sha256:8c247422…ecc673b` |
| Prompt | `unified_analyst_v1_29` / `0f19a402ebea7d7f5ddd28525ec3eca5e87209118352f6282db1e6a19c2b4c49` |
| Section policy | `section_fact_policy_v1` |

### What this fix solved (confirmed live)

| Check | Result |
| --- | --- |
| Offline run 204 finalize (drop campaign-only pulse item) | **PASS** |
| Live output validation | **PASS** (`publication_ready`) |
| Seller Pulse fact scopes | **account-only** (`campaign_in_pulse=0`) |
| Live `seller_pulse_ai_fact_mismatch` | **absent** |
| Risk drivers / Top Actions path | reached publish (not blocked) |
| Provider attempts | 1 (completed) |
| Pack hash | `17e80bbe…036be854` (deterministic) |

### Live run

```text
run_id: 205
status after validate: publication_ready
publication_id: 91
status after publish: publication_failed
error_code: advertising_total_revenue_share_mismatch
```

New product defect (unrelated to Seller Pulse section grounding):

```text
advertising_total_revenue_share_mismatch
field=ad_efficiency.narrative.summary
narrative store-revenue share 53.3% conflicts with facts (~26.6%)
```

## Stop rule

New product defect discovered at bundle validation. **Not fixed in commit `fdc847c`.** Full Playwright / GET fingerprints / rollback / pointer switch not run.

## Proposed next fix-commit (separate)

Ground / validate `ad_efficiency.narrative` store-revenue share percentages against Pack facts (and/or prompt rules forbidding mismatched store-share numbers).

## Evidence

- `unified-run.log`
- `inspect-output-205.txt`
- `inspect-publication-91.json` / `inspect-publication-74.json`
- `seller_pulse_fact_ids_205.json` — account-only facts
- `pointer-and-status.txt`
