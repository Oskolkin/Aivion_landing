import { renderToStaticMarkup } from "react-dom/server";
import { DashboardDataContext } from "@/components/dashboard/shell/dashboard-data-provider";
import { FunnelDashboardContent } from "@/components/dashboard/pages/dashboard-funnel-page";
import type { PublicationDashboardResponse } from "@/lib/publication-dashboard";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function wrap(data: PublicationDashboardResponse) {
  return renderToStaticMarkup(
    <DashboardDataContext.Provider value={{ data, loading: false, error: "", reload: () => undefined }}>
      <FunnelDashboardContent />
    </DashboardDataContext.Provider>,
  );
}

const base: PublicationDashboardResponse = {
  state: "READY",
  publication: {
    id: "1",
    analysis_date: "2026-08-31",
    analysis_cutoff_at: "2026-08-31T21:00:00Z",
    published_at: "2026-08-31T22:00:00Z",
  },
  funnel: {
    schema_version: "funnel_projection_v2",
    state: "READY",
    status: "READY",
    period: { current: { key: "current_7d" }, previous: { key: "previous_7d" } },
    stages: [
      { key: "PDP_IMPRESSIONS", status: "READY", amount: "100" },
      { key: "PDP_CART_ADDS", status: "READY", amount: "10" },
    ],
    transitions: [],
    insight: { headline: "Воронка", statement: "Цепочка совместима", primary_domain: "FUNNEL" },
    related_action_local_refs: ["a1"],
  },
};

function run() {
  const ready = wrap(base);
  assert(ready.includes("dashboard-funnel-page"), "funnel page");
  assert(ready.includes("funnel-visualization"), "visualization is dominant");
  assert(!ready.includes("funnel-kpi-"), "old KPI row removed");
  assert(!ready.includes("funnel-key-signals"), "old signals removed");
  assert(ready.includes("funnel-insight-card"), "insight card");
  assert(ready.includes("funnel-related-actions"), "related actions same publication");

  const empty = wrap({
    ...base,
    funnel: { ...base.funnel!, state: "READY_EMPTY", status: "READY_EMPTY", stages: [], transitions: [] },
  });
  assert(empty.includes("funnel-ready-empty"), "READY_EMPTY");
  assert(empty.includes("активности воронки не было"), "empty copy");

  const unavailable = wrap({
    ...base,
    funnel: {
      ...base.funnel!,
      state: "UNAVAILABLE",
      status: "UNAVAILABLE",
      reason_code: "NO_COMPATIBLE_CHAIN",
      stages: [],
      transitions: [],
    },
  });
  assert(unavailable.includes("funnel-unavailable"), "UNAVAILABLE");
  assert(!unavailable.includes("funnel-visualization"), "no fake stages");

  const failed = wrap({
    ...base,
    funnel: { ...base.funnel!, state: "FAILED", status: "FAILED", stages: [], transitions: [] },
  });
  assert(failed.includes("funnel-failed"), "FAILED");

  const partial = wrap({
    ...base,
    funnel: { ...base.funnel!, state: "PARTIAL", status: "PARTIAL" },
  });
  assert(partial.includes("Доступна часть воронки"), "PARTIAL copy");

  const history = wrap({
    ...base,
    funnel: { ...base.funnel!, limitations: ["INSUFFICIENT_HISTORY"] },
  });
  assert(history.includes("Недостаточно истории"), "INSUFFICIENT_HISTORY");

  console.log("funnel-dashboard-page.test.tsx: ok");
}

run();
