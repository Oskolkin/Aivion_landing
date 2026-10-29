# Dashboard acceptance history (2026-07)

Compact index of the Unified Dashboard acceptance chain. Detailed intermediate
evidence lives under `archive/dashboard-acceptance-2026-07/`.

| SHA | Gate | Outcome | Notes |
| --- | --- | --- | --- |
| `a18e153` | output validation | FAIL | Top Actions referenced `recommended` tier |
| `0dc225f` | bundle validation | FAIL | unknown dynamic risk-driver IDs |
| `e48186d` | output validation | FAIL | Seller Pulse campaign fact grounding |
| `fdc847c` | bundle validation | FAIL | `advertising_total_revenue_share_mismatch` (53.3 vs 26.6) |
| `8bbc469` | publication | **PASS** | claim-aware advertising bundle validation; run **206** / publication **92** |
| `1c7b784` | fixtures | PASS | browser fixture claim/cohort alignment |
| `dcfbc96` | evidence | PASS | acceptance archive + live pulse e2e for pub 92 |

Final report: `dashboard-final-real-run-acceptance-2026-07-22-8bbc469.md`
Active evidence: `real-run-2026-07-22-8bbc469/`
Advertising root-cause: `archive/.../fdc847c/advertising-bundle-claim-misclassification.md`
