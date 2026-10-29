# Dashboard real-run acceptance — 2026-07-22 — SHA 0dc225f

## Verdict

```text
Dashboard full audit acceptance FAILED on runtime SHA 0dc225f672ec7b27e30c6e6c5bbe1642e19a7891
```

| Field | Value |
| --- | --- |
| Exact gate | **Bundle validation / publication** |
| Exact error | `unified_dashboard_risk_driver_unknown_id` — AI `risk_drivers` referenced `high_drr_882000021528` and `low_stock_801000100001` outside backend healthpolicy registry |
| Evidence path | `docs/audits/real-run-2026-07-22-0dc225f/` |
| Pointer state | **unchanged** — current publication remains **74** |
| Previous Dashboard preserved | **yes** |

## Fix commit under test

```text
parent SHA:  a18e1538b0d506e3efbe6a141fc90b249e4ef9a1
fix SHA:     0dc225f672ec7b27e30c6e6c5bbe1642e19a7891
branch:      fix/provider-orchestration
```

### What the fix solved (confirmed live)

| Check | Result |
| --- | --- |
| Offline run 201 finalize + validate | PASS |
| Live output validation | **PASS** (`valid=true`) |
| `top_action_count` | **2** (required-only; no recommended pad) |
| Prompt | `unified_analyst_v1_27` / `ff70ed2316e09c9455a4b13e1a52a0720ef788c65c4fdf22320a59a6b99acc2f` |
| Schema | `aivion_unified_analysis_v10` / `sha256:8c247422…ecc673b` (unchanged) |
| Provider attempts | 1 (completed) |
| Pack hash | `17e80bbe…036be854` (deterministic) |

Run **202** reached `publication_ready` after output validation, then failed publishing as publication **90** (`publication_failed`) with bundle error above. Pointer did not switch.

## Stop rule

New product defect discovered at bundle validation. **Not fixed in commit `0dc225f`.** Full Playwright / GET fingerprints / rollback not run.

## Proposed next fix-commit (separate)

Address health risk_driver allowlist / sanitization:

- AI emitted entity-scoped driver IDs (`high_drr_<campaign>`, `low_stock_<sku>`) not present in backend `healthpolicy` registry.
- Options: expand registry to allow documented patterns, strip unknown drivers before bundle validation, and/or prompt guidance to use only registry driver IDs.

## Evidence

- `inspect-output-202.txt` — output validation PASS, top_action_count=2
- `inspect-publication-90.json` — failed publication
- `inspect-publication-74.json` — preserved current
- `pointer-and-status.txt`
- `unified-run.log`
