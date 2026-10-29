# Dashboard routes and Overview composition

## Routes

| Path | Page |
|------|------|
| `/app/dashboard` | Overview (Обзор) |
| `/app/dashboard/funnel` | Funnel detail (existing SalesFunnelModule) |
| `/app/dashboard/stock` | Stock detail (existing StockCoverageModule) |
| `/app/dashboard/advertising` | Advertising detail (existing AdEfficiencyModule) |
| `/app/dashboard/categories` | Categories (existing CategoryStatusCardsSection) |
| `/app/dashboard`#sku-focus | SKU Focus (existing SKUFocusSection, bottom of Overview) |
| `/app/dashboard/sku` | Redirects to Overview `#sku-focus` |

Shared shell: `components/dashboard/shell/dashboard-shell.tsx`  
Data: single `getDashboardHome()` via `DashboardDataProvider` (no duplicate publication fetch per tab).

AppShell section title strip is suppressed on `/app/dashboard/**` so only the Dashboard `PageHeader` shows «Дашборд».

## Overview components

- `OverviewKpiGrid` / `OverviewKpiCard` — analytic mini line charts with axes + tooltip
- `OverviewAiSummary` — no Actions CTA
- `OverviewFocusActions`
- `OverviewHealthIndex` — decline factors + domain icons, no impact scores / false CTA
- `OverviewWhatChanged` — labels: Новые риски / Актуальные риски
- Domain previews: mini funnel (CTR по кабинету), stock bar chart, dual DRR (7d + yesterday)

Mapping lives in `lib/overview-view-model.ts` (frontend-only presentation mapping over existing Dashboard DTO).

## Detail pages

Detail routes reuse current modules without visual redesign. Full analytics blocks are no longer stacked on Overview.
