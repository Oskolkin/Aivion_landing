# Dashboard full audit — implementation report (2026-07-21)

**Baseline commit:** `e0dfc3227e9d92b2a28a8590fec7090d651907a4`  
**Audit artifact:** `docs/audits/dashboard-full-audit-2026-07-21.md`

## Summary

Implemented the highest-impact ownership/consistency fixes from the full Dashboard audit: Seller Pulse KPIs now match the product formula (AOV + store ad share), health score is backend-owned at publication with breakdown, Unified GET no longer re-strengthens What Changed, and schema/prompt grounding bugs for pulse signals / `account_drr_pct` were fixed.

## Problems found → fixed

| Problem | Root cause | Fix |
| ------- | ---------- | --- |
| Pulse showed CTR instead of AOV | `dashboardfacts/seller_pulse.go` built CTR KPI | Emit `average_order_value` (null if orders=0) + sparkline |
| Pulse labeled store share as «ДРР» | key `drr` + RU label | key `ad_spend_share_total_revenue`, label «Реклама в общей выручке» |
| Health score AI-owned | adapter passed AI score through | `ComputeHealthScoreBreakdown(drivers)` overrides at `ProjectDashboard`; breakdown persisted |
| Breakdown hidden in production | `APP_ENV` gate | Always expose breakdown |
| What Changed rewritten on GET | `repairSellerFacingWhatChangedConclusion` | Skipped for Unified publication prompt version |
| Schema missing `seller_pulse_key_point` | v8 enum incomplete | Added to `unified_analysis_output_v8.schema.json` |
| Prompt said `account_drr_pct` = attributed DRR | Wrong grounding text | Aligned with Pack (store share alias) |

## Architecture after change

```text
Snapshot → DashboardFacts (AOV + ad share KPIs)
        → Pack → Unified Analyst (v8 / v1_22)
        → outputvalidator
        → ProjectDashboard:
             facts win for KPIs/funnel/stock/ads
             backend health score + breakdown from risk_drivers
             AI narratives / Top 3 / SKU selection
        → bundlevalidator → atomic publish
        → BuildHome: ApplyFrozen + narrative apply
             (no WC semantic strengthen on Unified)
```

## Field ownership (updated)

See `docs/current/dashboard-field-ownership.md`.

Notable: `health_score` / `health_score_breakdown` are now **DERIVED_FACT** at publish (from AI-selected risk drivers — stepping stone until drivers themselves are fact-built).

## Tests run

| Suite | Result |
| ----- | ------ |
| `go test ./internal/dashboardfacts/` | PASS |
| `go test ./internal/dashboardhome/` | PASS |
| `go test ./internal/unifiedanalysis/publication/adapter/` | PASS |
| `go test ./internal/unifiedanalysis/outputvalidator/` | PASS |
| `go test ./internal/unifiedanalysis/prompt/` | PASS |
| `go test ./internal/unifiedanalysis/contract/` | PASS |
| `npx tsx lib/seller-pulse-view.test.tsx` | PASS |
| `dashboard-typography.test.tsx` (via npm test) | PASS |

## Remaining limitations (honest)

1. **No new real Unified publication / browser E2E** in this session — existing frozen pubs still carry CTR/`drr` until re-published; FE maps legacy `drr`→ad share for read.
2. **Key SKU 80% cohort** still AI-selected on Unified path (`SKUFactRecord` lacks `revenue_7d` for adapter cohort rebuild).
3. **Health drivers** remain AI-selected; only the score formula is backend-deterministic given those drivers.
4. **Many GET-path presentation helpers** remain (labels, quick-action hrefs, surplus→slow_turnover, health status bands).
5. **Ad narrative % grounding rewrite** still mutates text at publish (prefer reject in a later pass).
6. **Category shells / recommended_actions** still in API as deprecated/optional.
7. **Full Playwright / real-run acceptance matrix** from §16–19 not re-executed end-to-end here.

## Key files changed

- `backend/internal/dashboardfacts/seller_pulse.go` (+ AOV test)
- `backend/internal/dashboardfacts/validate.go`
- `backend/internal/dashboardhome/*` (finalize, UI contract, health exposure, account health apply)
- `backend/internal/unifiedanalysis/publication/adapter/dashboard.go`
- `backend/internal/unifiedanalysis/contract/schema/unified_analysis_output_v8.schema.json`
- `backend/internal/unifiedanalysis/prompt/analyst_v1.txt`, `analyst_v1.go`
- `frontend/lib/seller-pulse-view.ts` (+ tests)
- `docs/current/dashboard-field-ownership.md`
- `docs/audits/dashboard-full-audit-2026-07-21.md`
