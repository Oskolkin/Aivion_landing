import { healthScoreColor, isWidgetAvailable, isWidgetRenderable, severityColor } from "@/components/dashboard/widget-state";
import { normalizeDashboardHome } from "@/lib/normalize-dashboard-home";
import type { DashboardHomeResponse } from "@/lib/dashboard-home-types";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function run() {
  assert(isWidgetAvailable("available"), "available");
  assert(isWidgetAvailable(undefined), "missing status treated available");
  assert(!isWidgetAvailable("partial"), "partial is not fully available");
  assert(isWidgetRenderable("partial"), "partial is renderable");
  assert(!isWidgetAvailable("error"), "error");
  assert(!isWidgetAvailable("not_enough_data"), "not_enough_data");
  assert(!isWidgetAvailable("source_not_available"), "source_not_available");
  assert(severityColor("attention").includes("text-[#0369a1]"), "attention severity color");
  assert(healthScoreColor(72) === "text-sky-700", "attention score color");

  const raw = {
    metric_date: "2026-03-10",
    status: "ready",
    data_quality_notes: null,
    ai_diagnostics: {
      account_summary: { status: "available" },
      what_changed: { status: "available", groups: null },
      top_actions: null,
      category_summaries: null,
      sales_funnel: { status: "available", current_completed_date: "", previous_completed_date: "", stages: null, daily_series: null },
      funnel_analysis: { status: "available", ai_analysis: { status: "not_enough_data" } },
      stock_coverage: { status: "available", segments: null },
      ad_efficiency: {
        status: "not_enough_data",
        message: "Рекламные кампании пока недоступны или не синхронизированы.",
        ad_efficiency_narrative: { status: "not_enough_data" },
        cta: { label: "Открыть рекламные кампании", href: "/app/advertising" },
      },
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
  } as unknown as DashboardHomeResponse;

  const normalized = normalizeDashboardHome(raw);
  assert(normalized.ai_diagnostics.ad_efficiency.period == null, "missing period stays absent");
  assert(normalized.ai_diagnostics.ad_efficiency.summary == null, "missing summary stays absent");
  assert(normalized.ai_diagnostics.ad_efficiency.status === "not_enough_data", "status preserved");

  console.log("widget-available.test.ts: ok");
}

run();
