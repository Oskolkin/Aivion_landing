import { asArray } from "@/lib/as-array";
import type { DashboardHomeResponse, SKUFocusCard, StockCoverageNarrative } from "@/lib/dashboard-home-types";
import { getStockCoverageActions, segmentCountByKey, type SegmentCountSource } from "@/lib/stock-coverage-display";
import { filterStockRiskItems, sortStockRiskItems, type StockRiskFilterKey } from "@/lib/sku-focus-display";

export type StockPageFilter = Exclude<StockRiskFilterKey, "all-risk"> | "all-attention" | "no-sales";

export const STOCK_PAGE_FILTERS: { key: StockPageFilter; label: string; segment?: string }[] = [
  { key: "out-of-stock", label: "Нет остатка", segment: "out_of_stock" },
  { key: "critical", label: "0–3 дня", segment: "critical_0_3" },
  { key: "short", label: "4–7 дней", segment: "shortage_4_7" },
  { key: "planning", label: "8–14 дней", segment: "planning_8_14" },
  { key: "no-sales", label: "Без продаж", segment: "no_sales" },
];

const attentionKeys = ["out_of_stock", "critical_0_3", "shortage_4_7", "planning_8_14"] as const;
const allKeys = [...attentionKeys, "healthy_14plus", "surplus", "no_sales", "collecting_data"] as const;

export type StockAttentionBucket = { key: (typeof attentionKeys)[number]; count: number };
export type StockAttentionChartLayout = { yAxisMax: number; yAxisTicks: number[] };

function niceStep(raw: number): number {
  if (raw <= 0) return 1;
  const magnitude = 10 ** Math.floor(Math.log10(raw));
  const normalized = raw / magnitude;
  const unit = normalized <= 1.5 ? 1 : normalized <= 3 ? 2 : normalized <= 7 ? 5 : 10;
  return unit * magnitude;
}

export function buildStockAttentionChartLayout(buckets: StockAttentionBucket[]): StockAttentionChartLayout {
  const maxCount = Math.max(0, ...buckets.map((bucket) => bucket.count));
  const step = niceStep(maxCount / 4);
  const yAxisMax = Math.max(step, Math.ceil(maxCount / step) * step);
  const yAxisTicks = Array.from({ length: Math.round(yAxisMax / step) + 1 }, (_, index) => index * step);
  return { yAxisMax, yAxisTicks };
}

function formatRevenueAtRiskValue(value?: string): string {
  const primary = (value ?? "—").split(/[·•]/u)[0]?.trim() || "—";
  return primary.replace(/₽\s*\/?\s*день/iu, "₽ / день");
}

export function buildStockPageViewModel(data: DashboardHomeResponse) {
  const coverage = data.ai_diagnostics.stock_coverage;
  const narrative = coverage.stock_coverage_narrative;
  const segments = (narrative?.segments
    ? asArray(narrative.segments)
    : asArray(coverage.segments)) as SegmentCountSource[];
  const count = (key: string) => segmentCountByKey(segments, key);
  const totalFromSegments = allKeys.reduce((sum, key) => sum + count(key), 0);
  const total = coverage.total_skus ?? totalFromSegments;
  const outOfStock = count("out_of_stock");
  const critical = count("critical_0_3");
  const shortage = count("shortage_4_7");
  const planning = count("planning_8_14");
  const urgentCount = outOfStock + critical;
  const supplyPlanCount = shortage + planning;
  const attentionCount = urgentCount + supplyPlanCount;
  const sufficientCoverageCount = Math.max(0, total - attentionCount);
  const descriptionParts = [
    outOfStock > 0 ? `${outOfStock} SKU без остатка` : "",
    critical > 0 ? `${critical} SKU с покрытием 0–3 дня` : "",
    shortage > 0 ? `${shortage} SKU с покрытием 4–7 дней` : "",
    planning > 0 ? `${planning} SKU с покрытием 8–14 дней` : "",
  ].filter(Boolean);

  return {
    narrative,
    total,
    totalSkus: total,
    totalFromSegments,
    hasReconciliationMismatch: total !== totalFromSegments,
    urgentCount,
    attentionCount,
    supplyPlanCount,
    healthyCount: sufficientCoverageCount,
    sufficientCoverageCount,
    hero: attentionCount > 0
      ? {
          title: `${attentionCount} SKU требуют внимания по остаткам`,
          description: `${descriptionParts.join(", ")}. Срочно пополните критичные позиции и запланируйте поставки по товарам с запасом до 14 дней.`,
        }
      : {
          title: "Критического дефицита не обнаружено",
          description: total > 0 ? `В ассортименте ${total} SKU: критичных остатков нет.` : "Недостаточно данных по остаткам.",
        },
    attentionBuckets: attentionKeys.map((key) => ({ key, count: count(key) })),
    reconciliation: { healthyCount: sufficientCoverageCount, noSalesCount: count("no_sales"), surplusCount: count("surplus"), collectingDataCount: count("collecting_data"), totalSkus: total },
    kpis: {
      critical: urgentCount,
      shortage: supplyPlanCount,
      revenue: narrative?.revenue_at_risk,
      revenueValue: formatRevenueAtRiskValue(narrative?.revenue_at_risk?.value),
    },
    actions: buildStockPageActions(narrative, data),
  };
}

export function getStockPageItems(data: DashboardHomeResponse, filter: StockPageFilter): SKUFocusCard[] {
  if (filter === "no-sales") return [...asArray(data.sku_focus.no_sales_with_stock)].sort((a, b) => (b.current_stock ?? 0) - (a.current_stock ?? 0));
  if (filter === "all-attention") {
    return sortStockRiskItems(asArray(data.sku_focus.stock_risk).filter((item) => {
      const bucket = item.coverage_bucket;
      return bucket === "out_of_stock" || bucket === "critical_0_3" || bucket === "shortage_4_7" || bucket === "planning_8_14";
    }), "all-risk");
  }
  return sortStockRiskItems(filterStockRiskItems(asArray(data.sku_focus.stock_risk), filter), filter);
}

export function getInitialStockPageFilter(data: DashboardHomeResponse): StockPageFilter {
  const order: StockPageFilter[] = ["critical", "out-of-stock", "short", "planning", "no-sales"];
  return order.find((filter) => getStockPageItems(data, filter).length > 0) ?? "critical";
}

function buildStockPageActions(narrative: StockCoverageNarrative | null | undefined, data: DashboardHomeResponse) {
  const narrativeActions = getStockCoverageActions(narrative);
  if (narrativeActions.length) return narrativeActions.map((action, index) => ({ ...action, priority: asArray(narrative?.actions ?? narrative?.checklist)[index]?.priority, target: asArray(narrative?.actions ?? narrative?.checklist)[index]?.target }));
  return asArray(data.top_actions)
    .filter((action) => action.category === "stock" || action.category === "slow_turnover")
    .slice(0, 4)
    .map((action) => ({ title: action.title, text: action.next_step || action.reason, priority: action.priority, target: undefined }));
}
