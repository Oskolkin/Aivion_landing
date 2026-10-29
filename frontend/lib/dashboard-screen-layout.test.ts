import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

function run() {
  const overviewPage = path.resolve(process.cwd(), "app/app/dashboard/page.tsx");
  const layout = path.resolve(process.cwd(), "app/app/dashboard/layout.tsx");
  const overviewSource = fs.readFileSync(overviewPage, "utf8");
  const layoutSource = fs.readFileSync(layout, "utf8");

  assert.ok(layoutSource.includes("DashboardShell"), "dashboard layout should use DashboardShell");
  assert.ok(
    overviewSource.includes("DashboardOverviewPage"),
    "dashboard root should render Overview page",
  );

  const detailRoutes = [
    "app/app/dashboard/funnel/page.tsx",
    "app/app/dashboard/stock/page.tsx",
    "app/app/dashboard/advertising/page.tsx",
    "app/app/dashboard/categories/page.tsx",
    "app/app/dashboard/sku/page.tsx",
  ];
  for (const route of detailRoutes) {
    const routePath = path.resolve(process.cwd(), route);
    assert.ok(fs.existsSync(routePath), `detail route should exist: ${route}`);
  }

  const skuRouteSource = fs.readFileSync(
    path.resolve(process.cwd(), "app/app/dashboard/sku/page.tsx"),
    "utf8",
  );
  assert.ok(skuRouteSource.includes('redirect("/app/dashboard#sku-focus")'), "SKU route should redirect to Overview focus");

  const overviewComp = path.resolve(
    process.cwd(),
    "components/dashboard/overview/dashboard-overview-page.tsx",
  );
  const overviewCompSource = fs.readFileSync(overviewComp, "utf8");
  assert.ok(overviewCompSource.includes("OverviewKpiGrid"), "overview should render KPI grid");
  assert.ok(overviewCompSource.includes("OverviewAiSummary"), "overview should render AI summary");
  assert.ok(
    overviewCompSource.includes("OverviewFocusActions"),
    "overview should render focus actions",
  );
  assert.ok(
    overviewCompSource.includes("OverviewWhatChanged"),
    "overview should render what changed",
  );
  assert.ok(
    overviewCompSource.includes("OverviewDomainPreviews"),
    "overview should render domain previews",
  );
  assert.ok(overviewCompSource.includes("<OverviewWhatChanged"), "dynamics and risks is a full-width section");
  assert.ok(!overviewCompSource.includes("OverviewHealthIndex"), "health index is removed from Overview DOM");

  assert.ok(overviewCompSource.includes("SKUFocusSection"), "overview should embed SKU focus");
  assert.ok(
    overviewCompSource.includes('<SKUFocusSection data={data.sku_focus} />'),
    "overview should render SKU focus from the existing dashboard payload",
  );
  assert.ok(
    !overviewCompSource.includes("CategoryStatusCardsSection"),
    "overview must not embed category cards",
  );
  assert.ok(
    !overviewCompSource.includes("SalesFunnelModule"),
    "overview must not embed full funnel module",
  );
  assert.ok(
    !overviewCompSource.includes("StockCoverageModule"),
    "overview must not embed full stock module",
  );
  assert.ok(
    !overviewCompSource.includes("AdEfficiencyModule"),
    "overview must not embed full ad module",
  );
  assert.ok(
    !overviewCompSource.includes("TopActionsSection"),
    "overview must not embed legacy top action cards",
  );

  console.log("dashboard-screen-layout tests passed");
}

run();
