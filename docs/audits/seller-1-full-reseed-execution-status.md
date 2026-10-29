# Seller 1 Full Reseed — Execution Status

**Date:** 2026-07-21  
**Status:** sections 27–33 completed for publication **74** / run **162**

## Confirmed identities

| Field | Value |
|-------|--------|
| seller_id | 1 |
| metric_date | 2026-07-08 |
| seed | 20260514 |
| products | 80 |
| pack_hash | `17e80bbe7eccb4f8cf7740e1d1ada796c22e4dd581b61c76874c38da036be854` |
| reconciliation_hash | `ed4017f2ee7773880fef2cde72af7bdf48e0094fb7786f5a4dba54366a91e9b9` |
| publication_id | 74 |
| run_id | 162 |
| output_hash | `337a48f80b1ca57572f4b6dc7990d6f5b53174a1c5aff7bcc498a8763fe2347a` |
| visibility | visible |
| content | ready / published |
| fallback_used | false (unified current) |

## Sections 27–33

| Step | Result | Notes |
|------|--------|-------|
| 27 Env + OpenAI preflight | PASS | Single API `:18081`, worker `:9091`; `check-ai-connectivity` status=ok |
| Failed run 161 | PASS | status=failed, cleanup=completed; not reused |
| Source revalidate | PASS | Valid=true, errors=0, warnings=0; recon hash unchanged |
| New provider run | PASS (162) | Provider completed; initial contract block fixed via grounding sanitizer + revalidate |
| Publish | PASS | publication 74, current unified pointer set |
| Persisted AI / Actions | PASS | actions_count=5, evidence 100% resolvable |
| `audit-unified-publication` | PASS | valid=true; 1 source **warning** (artifact hash serialization vs live); business hash still baseline |
| Playwright | PASS | dashboard-audit-export, unified-publication-smoke, full suite **12/12** |
| Final ZIP | PASS | `backend/debug/dashboard-audit-seller-1-pub-74-run-162.zip` |

## Notes / caveats

1. Run **161** failed with OpenAI `insufficient_quota` earlier; quota later restored (`check-ai-connectivity` ok).
2. Runs **163–168** were additional force attempts (contract / empty structured output). Publication uses **162**’s provider response after Stage-4 revalidate with `SanitizeGrounding` (pack identity, unknown/duplicate SKUs → account promote, unmet completion targets, signal_id dedupe).
3. Prompt current code is **`unified_analyst_v1_22`**; run 162 was produced under **`v1_21`** and published with `--allow-non-current-prompt`.
4. Source business reconciliation hash still `ed4017f2…`. Audit warning compares a different artifact-hash encoding, not the business recon hash.
5. No reset/reseed was performed in this phase.

## Evidence roots

- Working dir: `backend/debug/seller-1-full-reseed/`
- Audit: `backend/debug/seller-1-full-reseed/final-audit/`
- ZIP: `backend/debug/dashboard-audit-seller-1-pub-74-run-162.zip`
