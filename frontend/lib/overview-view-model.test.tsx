import assert from "node:assert/strict";
import { buildOverviewViewModel } from "@/lib/overview-view-model";
import type { PublicationDashboardResponse } from "@/lib/publication-dashboard";

function run() {
  const data: PublicationDashboardResponse = {
    state: "READY",
    publication: { id: "1", analysis_date: "2026-08-31", analysis_cutoff_at: "x", published_at: "y" },
    overview: {
      schema_version: "overview_projection_v2",
      seller_account_id: 1,
      analysis_date: "2026-08-31",
      analysis_cutoff_at: "x",
      kpis: [
        { key: "ORDERED_AMOUNT", status: "READY", amount: "1", unit: "RUB" },
        { key: "ORDERED_UNITS", status: "MISSING" },
        { key: "AD_SHARE", status: "READY", amount: "0.1", unit: "RATIO" },
      ],
      executive_summary: "s",
    },
  };
  const vm = buildOverviewViewModel(data);
  assert.equal(vm?.kpis.length, 3);
  assert.equal(vm?.kpis[1].valueDisplay, "—");
  console.log("overview-view-model tests passed");
}

run();
