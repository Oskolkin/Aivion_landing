import { asArray } from "@/lib/as-array";
import type { ActionItem, DashboardHomeResponse, RiskDriver, SKUFocusCard, TopActionItem } from "@/lib/dashboard-home-types";
import { normalizeSkuFocusImageUrl } from "@/lib/sku-focus-display";

function normalizeSKUFocusCards(cards: SKUFocusCard[] | null | undefined): SKUFocusCard[] {
  return asArray(cards).map((card) => {
    const imageUrl = normalizeSkuFocusImageUrl(card);
    return {
      ...card,
      image_url: imageUrl ?? card.image_url,
      sparkline_7d: asArray(card.sparkline_7d),
      stock_trend_7d: asArray(card.stock_trend_7d),
    };
  });
}

function normalizeActions(actions: ActionItem[] | null | undefined): ActionItem[] {
  return asArray(actions);
}

function normalizeTopActions(actions: TopActionItem[] | null | undefined): TopActionItem[] {
  return asArray(actions).map((action) => ({
    ...action,
    linked_alert_ids: asArray(action.linked_alert_ids),
    linked_recommendation_ids: asArray(action.linked_recommendation_ids),
    supporting_metrics: asArray(action.supporting_metrics),
  }));
}

function normalizeRiskDrivers(drivers: RiskDriver[] | null | undefined): RiskDriver[] {
  return asArray(drivers).map((driver) => ({
    ...driver,
    linked_alert_ids: asArray(driver.linked_alert_ids),
    linked_recommendation_ids: asArray(driver.linked_recommendation_ids),
  }));
}

/** Normalize dashboard home payload so UI never sees null array fields. */
export function normalizeDashboardHome(raw: DashboardHomeResponse): DashboardHomeResponse {
  const ai = raw.ai_diagnostics;
  const wc = ai.what_changed;
  const rootTopActions = normalizeTopActions(raw.top_actions);
  const aiTopActions = normalizeTopActions(ai.top_actions);
  const topActions = rootTopActions.length > 0 ? rootTopActions : aiTopActions;

  return {
    ...raw,
    top_actions: topActions,
    data_quality_notes: asArray(raw.data_quality_notes),
    ai_diagnostics: {
      ...ai,
      account_summary: {
        ...ai.account_summary,
        risk_drivers: normalizeRiskDrivers(ai.account_summary.risk_drivers),
        health_score_breakdown: ai.account_summary.health_score_breakdown
          ? {
              ...ai.account_summary.health_score_breakdown,
              drivers: asArray(ai.account_summary.health_score_breakdown.drivers),
            }
          : undefined,
      },
      what_changed: {
        ...wc,
        groups: asArray(wc.groups).map((group) => ({
          ...group,
          items: asArray(group.items).map((item) => ({
            ...item,
            highlights: asArray(item.highlights),
          })),
        })),
      },
      top_actions: topActions,
      top_actions_source: ai.top_actions_source ?? raw.pipeline?.top_actions_source,
      category_summaries: asArray(ai.category_summaries),
      sales_funnel: {
        ...ai.sales_funnel,
        stages: asArray(ai.sales_funnel.stages),
        daily_series: asArray(ai.sales_funnel.daily_series),
        overview_7d: ai.sales_funnel.overview_7d
          ? {
              ...ai.sales_funnel.overview_7d,
              stages: asArray(ai.sales_funnel.overview_7d.stages),
            }
          : undefined,
      },
      funnel_analysis: {
        ...ai.funnel_analysis,
        ai_analysis: {
          ...(ai.funnel_analysis.ai_analysis ?? { status: "not_enough_data" }),
          related_signals: asArray(ai.funnel_analysis.ai_analysis?.related_signals),
          recommended_focus: asArray(ai.funnel_analysis.ai_analysis?.recommended_focus),
          influence_factors: asArray(ai.funnel_analysis.ai_analysis?.influence_factors),
          main_conclusion: ai.funnel_analysis.ai_analysis?.main_conclusion,
          action_items: asArray(ai.funnel_analysis.ai_analysis?.action_items),
          drop_factors: asArray(ai.funnel_analysis.ai_analysis?.drop_factors),
          check_directions: asArray(ai.funnel_analysis.ai_analysis?.check_directions),
          details_sections: asArray(ai.funnel_analysis.ai_analysis?.details_sections).map((sec) => ({
            ...sec,
            items: asArray(sec?.items),
          })),
          comparisons_commentary: ai.funnel_analysis.ai_analysis?.comparisons_commentary,
          comparisons: {
            previous_period: asArray(ai.funnel_analysis.ai_analysis?.comparisons?.previous_period),
            seven_day_average: asArray(ai.funnel_analysis.ai_analysis?.comparisons?.seven_day_average),
            stage_metrics: asArray(ai.funnel_analysis.ai_analysis?.comparisons?.stage_metrics),
            conversion_metrics: asArray(ai.funnel_analysis.ai_analysis?.comparisons?.conversion_metrics),
            business_metrics: asArray(ai.funnel_analysis.ai_analysis?.comparisons?.business_metrics),
            context_signals: asArray(ai.funnel_analysis.ai_analysis?.comparisons?.context_signals),
          },
        },
      },
      stock_coverage: {
        ...ai.stock_coverage,
        segments: asArray(ai.stock_coverage.segments),
        stock_coverage_narrative: ai.stock_coverage.stock_coverage_narrative
          ? {
              ...ai.stock_coverage.stock_coverage_narrative,
              segments: asArray(ai.stock_coverage.stock_coverage_narrative.segments),
              risk_examples: asArray(ai.stock_coverage.stock_coverage_narrative.risk_examples),
            }
          : undefined,
      },
      ad_efficiency: {
        ...ai.ad_efficiency,
        points: asArray(ai.ad_efficiency.points),
        trend_7d: asArray(ai.ad_efficiency.trend_7d),
        high_drr_campaigns: asArray(ai.ad_efficiency.high_drr_campaigns),
        no_order_campaigns: asArray(ai.ad_efficiency.no_order_campaigns),
      },
      risk_distribution: {
        ...ai.risk_distribution,
        groups: asArray(ai.risk_distribution.groups),
      },
    },
    seller_pulse: {
      kpis: asArray(raw.seller_pulse.kpis).map((kpi) => ({
        ...kpi,
        sparkline_7d: asArray(kpi.sparkline_7d),
      })),
      trend_7d: asArray(raw.seller_pulse.trend_7d),
      narrative: raw.seller_pulse.narrative
        ? {
            ...raw.seller_pulse.narrative,
            important_notes: asArray(raw.seller_pulse.narrative.important_notes),
            key_points: asArray(raw.seller_pulse.narrative.key_points),
            quick_actions: asArray(raw.seller_pulse.narrative.quick_actions),
            chips: asArray(raw.seller_pulse.narrative.chips),
          }
        : undefined,
    },
    sku_focus: {
      order_drop: normalizeSKUFocusCards(raw.sku_focus.order_drop),
      stock_risk: normalizeSKUFocusCards(raw.sku_focus.stock_risk),
      no_sales_with_stock: normalizeSKUFocusCards(raw.sku_focus.no_sales_with_stock),
      no_sales_with_stock_total_count: raw.sku_focus.no_sales_with_stock_total_count ?? 0,
      surplus: normalizeSKUFocusCards(raw.sku_focus.surplus),
      key_skus: asArray(raw.sku_focus.key_skus).map((card) => ({
        ...card,
        image_url: card.image_url?.trim() || card.image_url,
        revenue_trend_7d: asArray(card.revenue_trend_7d),
      })),
      key_skus_total_count: raw.sku_focus.key_skus_total_count ?? 0,
      store_total_revenue_7d: raw.sku_focus.store_total_revenue_7d ?? 0,
      key_skus_total_revenue_7d: raw.sku_focus.key_skus_total_revenue_7d ?? 0,
    },
    recommended_actions: normalizeActions(raw.recommended_actions),
  };
}
