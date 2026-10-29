import { asArray } from "@/lib/as-array";
import { normalizeDashboardHome } from "@/lib/normalize-dashboard-home";
import type { DashboardHomeResponse } from "@/lib/dashboard-home-types";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

// Lightweight checks runnable via: npx tsx lib/as-array.test.ts
function run() {
  assert(asArray(null).length === 0, "null -> []");
  assert(asArray(undefined).length === 0, "undefined -> []");
  assert(asArray(["x"]).length === 1, "array preserved");

  const raw = {
    metric_date: "2026-05-20",
    status: "ready",
    data_quality_notes: null,
    ai_diagnostics: {
      account_summary: { status: "available" },
      what_changed: {
        status: "available",
        groups: null,
      },
      top_actions: null,
      category_summaries: null,
      sales_funnel: { status: "available", current_completed_date: "", previous_completed_date: "", stages: null, daily_series: null },
      funnel_analysis: {
        status: "available",
        ai_analysis: { status: "not_enough_data", related_signals: null, recommended_focus: null },
      },
      stock_coverage: { status: "available", segments: null },
      ad_efficiency: { status: "available", points: null },
      risk_distribution: { status: "available", groups: null },
    },
    seller_pulse: { kpis: null, trend_7d: null },
    sku_focus: {
      order_drop: null,
      stock_risk: null,
      no_sales_with_stock: null,
      surplus: null,
      key_skus: null,
    },
    recommended_actions: null,
  } as DashboardHomeResponse;

  const norm = normalizeDashboardHome(raw);
  const ai = norm.ai_diagnostics;
  const funnelAi = ai.funnel_analysis.ai_analysis!;
  assert(ai.what_changed.groups!.length === 0, "what_changed.groups normalized");
  assert(ai.top_actions!.length === 0, "top_actions normalized");
  assert(ai.category_summaries!.length === 0, "category_summaries normalized");
  assert(ai.sales_funnel.stages!.length === 0, "stages normalized");
  assert(ai.sales_funnel.daily_series!.length === 0, "daily_series normalized");
  assert(funnelAi.related_signals!.length === 0, "funnel related_signals normalized");
  assert(funnelAi.recommended_focus!.length === 0, "funnel recommended_focus normalized");
  assert(ai.risk_distribution.groups!.length === 0, "groups normalized");
  assert(norm.sku_focus.order_drop!.length === 0, "sku_focus normalized");
  assert(norm.recommended_actions!.length === 0, "recommended_actions normalized");

  console.log("as-array.test.ts: ok");
}

run();
