# Dashboard real-run acceptance — 2026-07-22 — SHA 8bbc469 (+ 1c7b784 fixtures)

## Verdict

```text
Dashboard advertising claim-aware bundle validation ACCEPTED on runtime SHA 8bbc469
(follow-up fixture alignment: 1c7b784)
```

| Field | Value |
| --- | --- |
| Exact gate previously failing | Bundle validation / publication |
| Previous error | `advertising_total_revenue_share_mismatch` (53.3% vs ~26.6%) |
| Live run | **206** |
| Publication | **92** `published` |
| Pointer | switched **74 → 92** (74 `superseded`) |
| Evidence | `docs/audits/real-run-2026-07-22-8bbc469/` |

## Commits

```text
parent SHA:  fdc847c98b09794b07749ecb66bb9575b91ed0ca
fix SHA:     8bbc469b72c1b76c2cd6d430998a554a991d27f2
fixture SHA: 1c7b784 (browser fixture claim/cohort alignment)
branch:      fix/provider-orchestration
```

| Contract | Value |
| --- | --- |
| Schema | `aivion_unified_analysis_v10` |
| Schema hash | `sha256:8c2474226f0de69ec18709df7da9853fa5fb11e00522caf829e270bc3ecc673b` |
| Prompt | `unified_analyst_v1_29` (unchanged) |
| Prompt hash | `0f19a402ebea7d7f5ddd28525ec3eca5e87209118352f6282db1e6a19c2b4c49` |

## Root cause

See `docs/audits/archive/dashboard-acceptance-2026-07/real-run-2026-07-22-fdc847c/advertising-bundle-claim-misclassification.md`.

Bundle validator regex-classified attributed DRR `53.3%` as store spend share because Cyrillic `\w` never matched `атрибутированный`, while claims already correctly labeled `attributed_campaign_drr_pct`.

## Live run 206 results

| Check | Result |
| --- | --- |
| Provider | PASS (1 attempt, completed) |
| Output validation | PASS |
| Seller Pulse grounding | PASS (account-level; store share 26.2%) |
| Advertising claims | `26.2%` → `ad_spend_share_of_total_store_revenue_pct`; `53.3%` → `attributed_campaign_drr_pct`; threshold 80 separate |
| Bundle validation | PASS |
| Publication | **92** published |
| Pointer switch | **74 → 92** atomic |
| Pack hash | `17e80bbe…036be854` (deterministic) |
| Dashboard/Actions identity | pub=92 run=206 both |
| Repeated GET fingerprints | stable |
| Publication audit | `valid=true` |
| Rollback 74 / restore 92 | PASS |
| Restart + reconcile | pointer 92, fingerprint unchanged |

## Offline replay run 205

Legacy heuristics reproduce `advertising_total_revenue_share_mismatch`; claim-aware path PASS (`TestOfflineReplayRun205_AdvertisingBundleClaimAware`).

## Tests

Backend targeted + unifiedanalysis/jobs/dashboardhome: PASS  
Frontend lint (0 errors), unit tests, build: PASS  
Playwright unified-publication-smoke: **6/6 PASS**  
Playwright full suite: PASS after updating live pulse expectation to pub 92 content  

## Prompt / schema

No bump — AI claims on run 205/206 were correct; defect was publication bundle validation only.
