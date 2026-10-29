import { asArray } from "@/lib/as-array";
import type {
  DashboardHomeResponse,
  FunnelDailyPoint as FunnelDailyContractPoint,
  FunnelStage,
} from "@/lib/dashboard-home-types";
import type { OverviewPeriodControlsView } from "@/lib/overview-view-model";
import {
  formatRUDateRangeWithYear,
  formatRUDayMonth,
  formatRUDayMonthYear,
} from "@/lib/period-context-display";

export type FunnelDailyPoint = {
  date: string;
  value: number;
};

export type FunnelDailyStageComparison = {
  points: FunnelDailyPoint[];
  current: FunnelDailyPoint | null;
  previous: FunnelDailyPoint | null;
  deltaPct: number | null;
};

function metricValue(point: FunnelDailyContractPoint, stageKey: string): number | null {
  switch (stageKey) {
    case "impressions":
      return point.impressions;
    case "product_views":
      return point.product_views;
    case "add_to_cart":
      return point.add_to_cart;
    case "orders":
      return point.ordered_units;
    case "delivered_units":
      return point.delivered_units;
    default:
      return null;
  }
}

/** Maps the one canonical backend daily_series field; missing observations stay missing. */
export function resolveFunnelDailyStageComparison(input: {
  salesFunnel: DashboardHomeResponse["ai_diagnostics"]["sales_funnel"];
  stage: FunnelStage;
  metricDate: string;
}): FunnelDailyStageComparison {
  const points = asArray(input.salesFunnel.daily_series)
    .filter((point) => point.date < input.metricDate)
    .flatMap((point) => {
      const value = metricValue(point, input.stage.key);
      return typeof value === "number" && Number.isFinite(value) ? [{ date: point.date, value }] : [];
    })
    .sort((left, right) => left.date.localeCompare(right.date));

  const current =
    typeof input.stage.value === "number" && input.salesFunnel.current_completed_date
      ? { date: input.salesFunnel.current_completed_date, value: input.stage.value }
      : null;
  const previous =
    typeof input.stage.previous_day_value === "number" && input.salesFunnel.previous_completed_date
      ? { date: input.salesFunnel.previous_completed_date, value: input.stage.previous_day_value }
      : null;

  return {
    points,
    current,
    previous,
    deltaPct:
      typeof input.stage.change_vs_previous_day_pct === "number"
        ? input.stage.change_vs_previous_day_pct
        : null,
  };
}

/** Funnel-only header semantics. Overview keeps its existing WoW period representation. */
export function buildFunnelPeriodControls(data: DashboardHomeResponse): OverviewPeriodControlsView {
  const funnel = data.ai_diagnostics.sales_funnel;
  const series = asArray(funnel.daily_series).filter((point) => point.date < data.metric_date);
  const firstDate = series.at(0)?.date ?? null;
  const lastDate = series.at(-1)?.date ?? null;
  const range =
    firstDate && lastDate
      ? firstDate === lastDate
        ? formatRUDayMonthYear(firstDate)
        : formatRUDateRangeWithYear(firstDate, lastDate)
      : null;
  const current = formatRUDayMonth(funnel.current_completed_date);
  const previous = formatRUDayMonth(funnel.previous_completed_date);
  const comparison = current && previous ? `${current} ↔ ${previous}` : null;
  const updatedAt = formatRUDayMonthYear(data.metric_date);
  const analysisPeriod = range ? `Динамика: ${range}` : null;
  const groups = [
    analysisPeriod ? { label: "Динамика", value: analysisPeriod } : null,
    comparison ? { label: "Сравнение", value: comparison } : null,
    updatedAt ? { label: "Обновлено", value: updatedAt } : null,
  ].filter((item): item is { label: string; value: string } => item != null);

  return {
    groups,
    analysisPeriod,
    comparisonPeriod: comparison,
    updatedAt,
    hasComparison: comparison != null,
  };
}
