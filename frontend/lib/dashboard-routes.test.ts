import assert from "node:assert/strict";
import {
  DASHBOARD_ROUTES,
  DASHBOARD_TABS,
  getDashboardSectionId,
  isDashboardPath,
  normalizeDashboardPath,
} from "@/lib/dashboard-routes";

function run() {
  assert.equal(normalizeDashboardPath("/app/dashboard/"), "/app/dashboard");
  assert.equal(getDashboardSectionId("/app/dashboard"), "overview");
  assert.equal(getDashboardSectionId("/app/dashboard/funnel"), "funnel");
  assert.equal(getDashboardSectionId("/app/dashboard/stock"), "stock");
  assert.equal(getDashboardSectionId("/app/dashboard/advertising"), "advertising");
  assert.equal(getDashboardSectionId("/app/dashboard/categories"), "overview");
  assert.equal(getDashboardSectionId("/app/dashboard/sku"), "overview");
  assert.ok(isDashboardPath("/app/dashboard/funnel"));
  assert.ok(!isDashboardPath("/app/actions"));
  assert.equal(DASHBOARD_ROUTES.overview, "/app/dashboard");
  assert.equal(DASHBOARD_TABS.length, 4);
  assert.deepEqual(
    DASHBOARD_TABS.map((t) => t.id),
    ["overview", "funnel", "stock", "advertising"],
  );
  console.log("dashboard-routes tests passed");
}

run();
