import type { DashboardHomeResponse, FunnelStage } from "@/lib/dashboard-home-types";

/** Minimal funnel stage for tests — fills required `label` from `key` when omitted. */
export function testFunnelStage(stage: Partial<FunnelStage> & Pick<FunnelStage, "key">): FunnelStage {
  return {
    label: stage.label ?? stage.key,
    ...stage,
  };
}

/** Partial dashboard home root for component tests. */
export function testDashboardHome(
  partial: Omit<Partial<DashboardHomeResponse>, "ai_diagnostics"> & {
    ai_diagnostics: Partial<DashboardHomeResponse["ai_diagnostics"]>;
  },
): DashboardHomeResponse {
  const aiDiagnostics: DashboardHomeResponse["ai_diagnostics"] = {
    account_summary: { status: "not_enough_data" },
    what_changed: { status: "not_enough_data", groups: [] },
    top_actions: [],
    category_summaries: [],
    sales_funnel: {
      status: "not_enough_data",
      current_completed_date: "",
      previous_completed_date: "",
      stages: [],
      daily_series: [],
    },
    funnel_analysis: { status: "not_enough_data" },
    stock_coverage: { status: "not_enough_data", segments: [] },
    ad_efficiency: { status: "not_enough_data", trend_7d: [] },
    risk_distribution: { status: "not_enough_data", groups: [] },
    ...partial.ai_diagnostics,
  };
  return {
    metric_date: partial.metric_date ?? "2026-03-01",
    status: partial.status ?? "ready",
    data_quality_notes: partial.data_quality_notes ?? null,
    seller_pulse: partial.seller_pulse ?? { kpis: [], trend_7d: [] },
    sku_focus: partial.sku_focus ?? {
      order_drop: [],
      stock_risk: [],
      no_sales_with_stock: [],
      surplus: [],
      key_skus: [],
    },
    recommended_actions: partial.recommended_actions ?? [],
    ...partial,
    ai_diagnostics: aiDiagnostics,
  };
}
