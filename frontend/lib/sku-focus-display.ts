import { asArray } from "@/lib/as-array";
import type { KeySkuCard, SKUFocusCard, SKUFocusGroupKey, SparklinePoint } from "@/lib/dashboard-home-types";
import { sparklineNumericValue } from "@/lib/sparkline-value";

export type SkuFocusKpiKey =
  | "orders_comparison"
  | "orders_delta"
  | "revenue"
  | "revenue_share"
  | "revenue_7d"
  | "stock"
  | "coverage"
  | "revenue_at_risk"
  | "no_sales_days"
  | "frozen_stock_value";

export type SkuFocusKpiItem = {
  key: string;
  label: string;
  sublabel?: string;
  value: string;
  tone?: "default" | "negative" | "positive" | "muted" | "warning" | "attention";
};

export type SkuFocusTrendPoint = {
  date: string;
  orders: number | null;
  revenue: number | null;
  stock: number | null;
  coverageDays: number | null;
  hasData: boolean;
};

export type SkuFocusTrendVariant = "orders" | "stock" | "revenue";

export type StockRiskFilterKey = "all-risk" | "out-of-stock" | "critical" | "short" | "planning";

export type CanonicalStockSegment =
  | "out_of_stock"
  | "critical_0_3"
  | "shortage_4_7"
  | "planning_8_14"
  | "healthy_14plus"
  | "surplus"
  | "no_sales"
  | "collecting_data";

type SkuFocusKpiCandidate = {
  key: SkuFocusKpiKey;
  label: string;
  sublabel?: string;
  value: string | null;
  tone?: "default" | "negative" | "positive" | "muted" | "warning" | "attention";
};

export type SkuFocusScenarioBadge = {
  label: string;
  className: string;
};

/** User-facing SKU-фокус tabs (surplus is kept in API/types but hidden from the block). */
export const VISIBLE_SKU_FOCUS_TAB_KEYS = [
  "order_drop",
  "key_skus",
] as const satisfies readonly SKUFocusGroupKey[];

const VISIBLE_SKU_FOCUS_TAB_SET = new Set<SKUFocusGroupKey>(VISIBLE_SKU_FOCUS_TAB_KEYS);

export function isVisibleSkuFocusTab(tab: SKUFocusGroupKey): boolean {
  return VISIBLE_SKU_FOCUS_TAB_SET.has(tab);
}

export function resolveVisibleSkuFocusTab(
  tab: SKUFocusGroupKey | null | undefined,
  fallback: SKUFocusGroupKey = "order_drop",
): SKUFocusGroupKey {
  if (tab && isVisibleSkuFocusTab(tab)) {
    return tab;
  }
  return fallback;
}

const TAB_LABELS: Record<SKUFocusGroupKey, string> = {
  order_drop: "Заказы",
  stock_risk: "Остатки",
  no_sales_with_stock: "Без продаж",
  surplus: "Излишки",
  key_skus: "Ключевые",
};

const KPI_ORDER_BY_TAB: Record<SKUFocusGroupKey, SkuFocusKpiKey[]> = {
  order_drop: ["orders_comparison", "orders_delta", "revenue"],
  stock_risk: ["stock", "coverage", "revenue_at_risk"],
  no_sales_with_stock: ["stock", "no_sales_days", "frozen_stock_value"],
  surplus: ["stock", "revenue"],
  key_skus: ["revenue_share", "revenue_7d", "coverage"],
};

const SCENARIO_BADGES: Record<SKUFocusGroupKey, SkuFocusScenarioBadge> = {
  order_drop: {
    label: "Падение заказов",
    className: "border-[#FECACA] bg-[#FFF1F2] text-[#DC2626]",
  },
  stock_risk: {
    label: "Риск по остатку",
    className: "border-[#FED7AA] bg-[#FFF7ED] text-[#EA580C]",
  },
  no_sales_with_stock: {
    label: "Без продаж",
    className: "border-[#E2E8F0] bg-[#F8FAFC] text-[#475569]",
  },
  surplus: {
    label: "Излишки",
    className: "border-[#FDE68A] bg-[#FFFBEB] text-[#D97706]",
  },
  key_skus: {
    label: "Ключевые SKU",
    className: "border-[#BFDBFE] bg-[#EFF6FF] text-[#1D4ED8]",
  },
};

const CHECK_FALLBACKS: Record<SKUFocusGroupKey, string> = {
  order_drop: "Проверьте карточку товара, цену, остатки и источники видимости.",
  stock_risk: "Проверьте остатки, скорость продаж и сроки поставки.",
  no_sales_with_stock: "Проверьте остаток, цену, доступность и участие в кампании.",
  surplus: "Проверьте оборачиваемость, цену и рекламные кампании по SKU.",
  key_skus: "Проверьте наличие ключевых SKU, актуальность цен и поддерживающую рекламу.",
};

const CHECK_HINT_KEYS = [
  "reason",
  "check_hint",
  "next_step",
  "recommended_action",
  "note",
  "explanation",
  "summary",
  "driver",
] as const;

const ORDER_DELTA_RE = /[ΔΔ]\s*заказ[аы]?\s*:\s*([+-]?\d+)/i;

const ORDERS_DELTA_KEYS = ["orders_delta_day_to_day", "orders_delta"] as const;
const ORDERS_PREVIOUS_KEYS = ["previous_orders", "orders_previous", "orders_before"] as const;
const ORDERS_CURRENT_KEYS = ["current_orders", "orders_current", "orders_after"] as const;

const REVENUE_DELTA_KEYS = [
  "revenue_delta_day_to_day",
  "revenue_delta",
  "revenue_change",
  "revenue_loss",
  "revenue_impact",
  "lost_revenue",
] as const;

const IMAGE_URL_KEYS = [
  "image_url",
  "imageUrl",
  "primary_image",
  "primaryImage",
  "primary_image_url",
  "product_image",
  "image",
] as const;

const ORDERS_DYNAMICS_LABEL = "Динамика заказов";
const WEEK_TO_WEEK_SUBLABEL = "к предыдущим 7 дням";
const DAY_TO_DAY_SUBLABEL = WEEK_TO_WEEK_SUBLABEL;

const KPI_LABELS: Record<SkuFocusKpiKey, string> = {
  orders_comparison: "Заказы",
  orders_delta: ORDERS_DYNAMICS_LABEL,
  revenue: "Выручка",
  revenue_share: "Доля в выручке",
  revenue_7d: "Выручка за 7 дней",
  stock: "Остаток",
  coverage: "Покрытие",
  revenue_at_risk: "Выручка под угрозой",
  no_sales_days: "Дней без продаж",
  frozen_stock_value: "Заморожено в остатках",
};

const REVENUE_AT_RISK_SUBLABEL = "за 7 дней";

const TREND_DATE_FORMATTER = new Intl.DateTimeFormat("ru-RU", {
  day: "numeric",
  month: "long",
});

export function getSkuFocusTabLabel(tab: SKUFocusGroupKey): string {
  return TAB_LABELS[tab];
}

export function hasSkuFocusRevenueMetrics(items: SKUFocusCard[]): boolean {
  return items.some((item) => getConfirmedRevenueDelta(item) != null);
}

export function buildSkuFocusSummaryText(
  tab: SKUFocusGroupKey,
  count: number,
  items: SKUFocusCard[] = [],
  options?: {
    keyRevenueSharePct?: number | null;
    stockShown?: number;
    stockTotal?: number;
    stockFilter?: StockRiskFilterKey;
  },
): string {
  const n = String(count);
  switch (tab) {
    case "order_drop":
      if (hasSkuFocusRevenueMetrics(items)) {
        return `${n} SKU просели по заказам и выручке. Сначала — товары с самым сильным падением заказов.`;
      }
      return `${n} SKU просели по заказам. Сначала — товары с самым сильным падением заказов по доступным данным.`;
    case "stock_risk":
      return buildStockRiskSummaryText(count, options?.stockFilter ?? "all-risk");
    case "no_sales_with_stock":
      if (count > 6) {
        return `${n} SKU без продаж 14+ дней при наличии остатка. Сначала — товары с наибольшей суммой в остатках.`;
      }
      return `${n} SKU без продаж 14+ дней при наличии остатка.`;
    case "surplus":
      return `${n} SKU с возможными излишками.`;
    case "key_skus": {
      if (count <= 0) {
        return "Недостаточно данных для определения ключевых SKU за последние 7 дней.";
      }
      const share = options?.keyRevenueSharePct;
      const shareLabel =
        share != null && Number.isFinite(share) && share > 0
          ? formatExactSharePct(share)
          : null;
      if (shareLabel) {
        if (count > 6) {
          return `${n} SKU формируют ${shareLabel} выручки за последние 7 дней. Сначала — товары с наибольшей долей в выручке.`;
        }
        return `${n} SKU формируют ${shareLabel} выручки за последние 7 дней.`;
      }
      if (count > 6) {
        return `${n} ключевых SKU по выручке за последние 7 дней. Сначала — товары с наибольшей долей в выручке.`;
      }
      return `${n} ключевых SKU по выручке за последние 7 дней.`;
    }
    default:
      return `${n} SKU в фокусе по направлению «${getSkuFocusTabLabel(tab)}».`;
  }
}

export function buildStockRiskSummaryText(count: number, filter: StockRiskFilterKey): string {
  const n = String(count);
  switch (filter) {
    case "out-of-stock":
      return `${n} SKU уже отсутствуют на складе. В первую очередь проверьте товары с недавними продажами и наибольшей потенциальной потерей выручки.`;
    case "critical":
      return `${n} SKU имеют критический запас до трёх дней. Эти товары необходимо включить в ближайшее пополнение.`;
    case "short":
      return `${n} SKU имеют короткий запас от четырёх до семи дней. Их нужно заранее включить в план поставки.`;
    case "planning":
      return `${n} SKU имеют покрытие от восьми до четырнадцати дней. Эти товары находятся под контролем и требуют планирования следующей поставки.`;
    case "all-risk":
    default:
      return `${n} SKU требуют внимания по остаткам. Сначала показаны товары с наиболее низким покрытием и максимальной выручкой под риском.`;
  }
}

export function getStockRiskFilterEmptyText(): string {
  return "В выбранном диапазоне SKU не обнаружены";
}

function formatExactSharePct(pct: number): string {
  const rounded = Math.round(pct * 10) / 10;
  return (
    new Intl.NumberFormat("ru-RU", {
      minimumFractionDigits: Number.isInteger(rounded) ? 0 : 1,
      maximumFractionDigits: 1,
    }).format(rounded) + "%"
  );
}

export function getSkuFocusEmptyStateTitle(tab: SKUFocusGroupKey): string {
  if (tab === "order_drop") {
    return "Нет SKU с подтверждённой просадкой заказов за выбранный период.";
  }
  if (tab === "stock_risk") {
    return "Нет SKU с низким покрытием по остаткам за выбранный период.";
  }
  if (tab === "no_sales_with_stock") {
    return "Нет SKU без продаж 14+ дней при наличии остатка.";
  }
  if (tab === "key_skus") {
    return "Недостаточно данных для определения ключевых SKU за последние 7 дней.";
  }
  return "В этой группе проблемных SKU не найдено.";
}

export function getSkuFocusScenarioBadge(tab: SKUFocusGroupKey): SkuFocusScenarioBadge | null {
  if (tab === "stock_risk" || tab === "order_drop" || tab === "no_sales_with_stock" || tab === "key_skus") {
    return null;
  }
  return SCENARIO_BADGES[tab];
}

export function getSkuFocusTrendVariant(tab: SKUFocusGroupKey): SkuFocusTrendVariant {
  if (tab === "stock_risk") return "stock";
  if (tab === "key_skus") return "revenue";
  return "orders";
}

export function getSkuFocusTrendLabel(tab: SKUFocusGroupKey): string {
  if (tab === "stock_risk") return "Остаток, 7 дней";
  if (tab === "key_skus") return "Выручка, 7 дней";
  return "Тренд заказов, 7 дней";
}

export function shouldShowSkuFocusCheckSection(tab: SKUFocusGroupKey): boolean {
  return tab !== "order_drop" && tab !== "stock_risk" && tab !== "no_sales_with_stock" && tab !== "key_skus";
}

export function shouldShowSkuFocusTrend(tab: SKUFocusGroupKey): boolean {
  return tab !== "no_sales_with_stock";
}

export function getStockRiskTotalCount(
  data: { stock_risk_total_count?: number; stock_risk?: SKUFocusCard[] | null } | null | undefined,
  visibleCount: number,
): number {
  const total = data?.stock_risk_total_count;
  if (typeof total === "number" && Number.isFinite(total) && total > 0) {
    return total;
  }
  return visibleCount;
}

export function getKeySkuRevenueSharePct(data: {
  store_total_revenue_7d?: number;
  key_skus_total_revenue_7d?: number;
  key_skus?: KeySkuCard[] | null;
} | null | undefined): number | null {
  const store = data?.store_total_revenue_7d;
  const key = data?.key_skus_total_revenue_7d;
  if (typeof store === "number" && store > 0 && typeof key === "number" && key >= 0) {
    return Math.round((key / store) * 1000) / 10;
  }
  const cards = data?.key_skus ?? [];
  let lastCum: number | null = null;
  for (const card of cards) {
    const cum = card.cumulative_revenue_share_pct;
    if (typeof cum === "number" && Number.isFinite(cum)) {
      lastCum = cum;
    }
  }
  return lastCum;
}

export function getNoSalesWithStockTotalCount(
  data: { no_sales_with_stock_total_count?: number; no_sales_with_stock?: SKUFocusCard[] | null } | null | undefined,
  visibleCount: number,
): number {
  const total = data?.no_sales_with_stock_total_count;
  if (typeof total === "number" && Number.isFinite(total) && total > 0) {
    return total;
  }
  return visibleCount;
}

export function getNoSalesWithStockViewAllHref(): string | null {
  return null;
}

export function getKeySkusTotalCount(
  data: { key_skus_total_count?: number; key_skus?: KeySkuCard[] | null } | null | undefined,
  visibleCount: number,
): number {
  const total = data?.key_skus_total_count;
  if (typeof total === "number" && Number.isFinite(total) && total > 0) {
    return total;
  }
  return visibleCount;
}

export function getKeySkusViewAllHref(): string | null {
  return null;
}

export function normalizeSkuFocusImageUrl(item: SKUFocusCard): string | null {
  const record = item as SKUFocusCard & Record<string, unknown>;
  if (item.image_url?.trim()) {
    return item.image_url.trim();
  }
  for (const key of IMAGE_URL_KEYS) {
    const value = record[key];
    if (typeof value === "string" && value.trim().length > 0) {
      return value.trim();
    }
  }
  return null;
}

export function getSkuFocusTrendPoints(item: SKUFocusCard): SparklinePoint[] {
  return asArray(item.sparkline_7d);
}

export function normalizeSkuFocusTrendPoints(
  item: SKUFocusCard,
  tab: SKUFocusGroupKey = "order_drop",
): SkuFocusTrendPoint[] {
  if (tab === "stock_risk") {
    return normalizeSkuFocusStockTrendPoints(item);
  }
  return getSkuFocusTrendPoints(item).map((point) => {
    const hasData = sparklinePointHasData(point);
    return {
      date: point.date,
      orders: hasData ? sparklineNumericValue(point.value) : null,
      revenue:
        point.revenue != null && Number.isFinite(point.revenue) ? point.revenue : null,
      stock: null,
      coverageDays: null,
      hasData,
    };
  });
}

export function normalizeKeySkuTrendPoints(item: KeySkuCard): SkuFocusTrendPoint[] {
  return asArray(item.revenue_trend_7d).map((point) => {
    const hasData = point.has_data !== false;
    return {
      date: point.date,
      orders: hasData && Number.isFinite(point.orders) ? point.orders : null,
      revenue: hasData && Number.isFinite(point.revenue) ? point.revenue : null,
      stock: null,
      coverageDays: null,
      hasData,
    };
  });
}

export function hasKeySkuTrendData(item: KeySkuCard): boolean {
  const points = asArray(item.revenue_trend_7d);
  let dataPoints = 0;
  for (const point of points) {
    if (point.has_data === false) continue;
    if (Number.isFinite(point.revenue)) {
      dataPoints += 1;
    }
  }
  return dataPoints >= 2;
}

export function normalizeSkuFocusStockTrendPoints(item: SKUFocusCard): SkuFocusTrendPoint[] {
  return asArray(item.stock_trend_7d).map((point) => {
    const hasData = point.has_data !== false;
    return {
      date: point.date,
      orders: null,
      revenue: null,
      stock: hasData && Number.isFinite(point.stock) ? point.stock : null,
      coverageDays:
        point.coverage_days != null && Number.isFinite(point.coverage_days)
          ? point.coverage_days
          : null,
      hasData,
    };
  });
}

function sparklinePointHasData(point: SparklinePoint): boolean {
  if (point.has_data === false) {
    return false;
  }
  if (point.has_data === true) {
    return true;
  }
  return sparklineNumericValue(point.value) != null;
}

export function formatSkuFocusTrendDate(date: string): string {
  const parsed = new Date(`${date}T12:00:00`);
  if (Number.isNaN(parsed.getTime())) {
    return date;
  }
  return TREND_DATE_FORMATTER.format(parsed);
}

export function formatSkuFocusTrendRevenue(value: number | null): string | null {
  if (value == null) {
    return null;
  }
  return `${new Intl.NumberFormat("ru-RU", {
    maximumFractionDigits: 0,
  }).format(value)} ₽`;
}

export function buildSkuFocusTrendTooltipText(
  point: SkuFocusTrendPoint,
  variant: SkuFocusTrendVariant = "orders",
): string {
  const dateLabel = formatSkuFocusTrendDate(point.date);
  if (variant === "stock") {
    const stockLabel = point.stock == null ? "нет данных" : `${point.stock} шт.`;
    const coverageLabel =
      point.coverageDays == null
        ? "нет данных"
        : `${formatCoverageDays(point.coverageDays)} дн.`;
    return `${dateLabel}\nОстаток: ${stockLabel}\nПокрытие: ${coverageLabel}`;
  }
  const ordersLabel = point.orders == null ? "нет данных" : String(point.orders);
  const revenueLabel =
    point.revenue == null || Number.isNaN(point.revenue)
      ? "нет данных"
      : formatSkuFocusTrendRevenue(point.revenue);
  return `${dateLabel}\nЗаказы: ${ordersLabel}\nВыручка: ${revenueLabel}`;
}

export function hasSkuFocusTrendData(item: SKUFocusCard, tab: SKUFocusGroupKey = "order_drop"): boolean {
  const status = String(item.trend_status ?? "").trim();
  if (status === "not_enough_data" || status === "source_not_available") {
    return false;
  }
  if (tab === "stock_risk") {
    return hasDrawableStockTrend(asArray(item.stock_trend_7d));
  }
  return hasDrawableSparkline(getSkuFocusTrendPoints(item));
}

export function getSkuFocusTrendEmptyMessage(item: SKUFocusCard): string {
  const msg = String(item.trend_message ?? "").trim();
  if (msg) {
    return msg;
  }
  const status = String(item.trend_status ?? "").trim();
  if (status === "not_enough_data") {
    return "Недостаточно точек для тренда";
  }
  if (status === "source_not_available") {
    return "Тренд недоступен";
  }
  return "Нет данных для тренда";
}

export function hasDrawableStockTrend(points: { stock: number; has_data?: boolean }[]): boolean {
  let dataPoints = 0;
  for (const point of points) {
    if (point.has_data === false) {
      continue;
    }
    if (Number.isFinite(point.stock)) {
      dataPoints += 1;
    }
  }
  return dataPoints >= 2;
}

export function hasDrawableSparkline(points: SparklinePoint[]): boolean {
  let dataPoints = 0;
  for (const point of points) {
    if (!sparklinePointHasData(point)) {
      continue;
    }
    dataPoints += 1;
  }
  return dataPoints >= 2;
}

export function getConfirmedOrdersDelta(item: SKUFocusCard): number | null {
  const record = item as SKUFocusCard & Record<string, unknown>;
  const structured = readNumericField(record, ORDERS_DELTA_KEYS);
  if (structured != null) {
    return structured;
  }
  return parseOrderDeltaFromKeyMetric(item.key_metric);
}

export function getConfirmedRevenueDelta(item: SKUFocusCard): number | null {
  const record = item as SKUFocusCard & Record<string, unknown>;
  return readNumericField(record, REVENUE_DELTA_KEYS);
}

export function sortOrderDropItems(items: SKUFocusCard[]): SKUFocusCard[] {
  return [...items].sort((left, right) => {
    const leftOrdersDelta = getConfirmedOrdersDelta(left);
    const rightOrdersDelta = getConfirmedOrdersDelta(right);

    if (leftOrdersDelta != null && rightOrdersDelta != null && leftOrdersDelta !== rightOrdersDelta) {
      return leftOrdersDelta - rightOrdersDelta;
    }
    if (leftOrdersDelta != null && rightOrdersDelta == null) {
      return -1;
    }
    if (leftOrdersDelta == null && rightOrdersDelta != null) {
      return 1;
    }

    const leftRevenueDelta = getConfirmedRevenueDelta(left);
    const rightRevenueDelta = getConfirmedRevenueDelta(right);
    if (
      leftRevenueDelta != null &&
      rightRevenueDelta != null &&
      leftRevenueDelta !== rightRevenueDelta
    ) {
      return leftRevenueDelta - rightRevenueDelta;
    }

    const nameCompare = left.product_name.localeCompare(right.product_name, "ru");
    if (nameCompare !== 0) {
      return nameCompare;
    }
    return left.ozon_product_id - right.ozon_product_id;
  });
}

export function getConfirmedCoverageDays(item: SKUFocusCard): number | null {
  if (item.coverage_days != null && Number.isFinite(item.coverage_days) && item.coverage_days > 0) {
    return item.coverage_days;
  }
  const parsed = parseMetricValue(item.key_metric, /покрытие\s+([\d.,]+)/i);
  if (!parsed) return null;
  const normalized = Number.parseFloat(parsed.replace(",", "."));
  return Number.isFinite(normalized) && normalized > 0 ? normalized : null;
}

export function getConfirmedRevenueAtRisk7d(item: SKUFocusCard): number | null {
  if (item.revenue_at_risk_7d != null && Number.isFinite(item.revenue_at_risk_7d) && item.revenue_at_risk_7d > 0) {
    return item.revenue_at_risk_7d;
  }
  return null;
}

export const STOCK_RISK_FILTER_ORDER: StockRiskFilterKey[] = [
  "all-risk",
  "out-of-stock",
  "critical",
  "short",
  "planning",
];

const STOCK_RISK_FILTER_LABELS: Record<StockRiskFilterKey, string> = {
  "all-risk": "Все риски",
  "out-of-stock": "Нет остатка",
  critical: "0–3 дня",
  short: "4–7 дней",
  planning: "8–14 дней",
};

const FILTER_TO_SEGMENT: Record<Exclude<StockRiskFilterKey, "all-risk">, CanonicalStockSegment> = {
  "out-of-stock": "out_of_stock",
  critical: "critical_0_3",
  short: "shortage_4_7",
  planning: "planning_8_14",
};

const ATTENTION_SEGMENTS = new Set<CanonicalStockSegment>([
  "out_of_stock",
  "critical_0_3",
  "shortage_4_7",
]);

const SEGMENT_URGENCY: Record<string, number> = {
  out_of_stock: 0,
  critical_0_3: 1,
  shortage_4_7: 2,
  planning_8_14: 3,
};

export function getStockRiskFilterLabel(filter: StockRiskFilterKey): string {
  return STOCK_RISK_FILTER_LABELS[filter];
}

function getConfirmedCurrentStock(item: SKUFocusCard): number | null {
  const current =
    item.current_stock != null && Number.isFinite(item.current_stock)
      ? item.current_stock
      : readNumericField(item as SKUFocusCard & Record<string, unknown>, ["current_stock"]);
  return current != null && Number.isFinite(current) ? current : null;
}

function getConfirmedRevenue7d(item: SKUFocusCard): number | null {
  const record = item as SKUFocusCard & Record<string, unknown>;
  const revenue = readNumericField(record, ["revenue_7d", "revenue"]);
  return revenue != null && Number.isFinite(revenue) ? revenue : null;
}

/** Resolve canonical segment from backend; fallback only for legacy payloads. */
export function getCanonicalStockSegment(item: SKUFocusCard): CanonicalStockSegment | null {
  const raw = String(item.coverage_bucket ?? "").trim();
  if (
    raw === "out_of_stock" ||
    raw === "critical_0_3" ||
    raw === "shortage_4_7" ||
    raw === "planning_8_14" ||
    raw === "healthy_14plus" ||
    raw === "surplus" ||
    raw === "no_sales" ||
    raw === "collecting_data"
  ) {
    return raw;
  }
  // Legacy fallback — prefer backend coverage_bucket whenever present.
  const stock = getConfirmedCurrentStock(item);
  if (stock != null && stock <= 0) {
    return "out_of_stock";
  }
  const coverage = getConfirmedCoverageDays(item);
  if (coverage == null) return null;
  if (coverage <= 3) return "critical_0_3";
  if (coverage <= 7) return "shortage_4_7";
  if (coverage <= 14) return "planning_8_14";
  return "healthy_14plus";
}

/** Buckets a stock-risk card by canonical segment for UI filters. */
export function classifyStockRiskBucket(
  item: SKUFocusCard,
): Exclude<StockRiskFilterKey, "all-risk"> | null {
  const segment = getCanonicalStockSegment(item);
  if (segment == null) return null;
  switch (segment) {
    case "out_of_stock":
      return "out-of-stock";
    case "critical_0_3":
      return "critical";
    case "shortage_4_7":
      return "short";
    case "planning_8_14":
      return "planning";
    default:
      return null;
  }
}

export function filterStockRiskItems(items: SKUFocusCard[], filter: StockRiskFilterKey): SKUFocusCard[] {
  if (filter === "all-risk") {
    return items.filter((item) => {
      const segment = getCanonicalStockSegment(item);
      return segment != null && ATTENTION_SEGMENTS.has(segment);
    });
  }
  const wanted = FILTER_TO_SEGMENT[filter];
  return items.filter((item) => getCanonicalStockSegment(item) === wanted);
}

export function countStockRiskItems(items: SKUFocusCard[], filter: StockRiskFilterKey): number {
  return filterStockRiskItems(items, filter).length;
}

export function sortStockRiskItems(
  items: SKUFocusCard[],
  filter: StockRiskFilterKey = "all-risk",
): SKUFocusCard[] {
  return [...items].sort((left, right) => {
    if (filter === "all-risk" || filter === "out-of-stock" || filter === "critical" || filter === "short") {
      const leftSeg = getCanonicalStockSegment(left) ?? "";
      const rightSeg = getCanonicalStockSegment(right) ?? "";
      const leftU = SEGMENT_URGENCY[leftSeg] ?? 9;
      const rightU = SEGMENT_URGENCY[rightSeg] ?? 9;
      if (leftU !== rightU) return leftU - rightU;
    }

    const leftCoverage = getConfirmedCoverageDays(left);
    const rightCoverage = getConfirmedCoverageDays(right);
    if (leftCoverage != null && rightCoverage != null && leftCoverage !== rightCoverage) {
      return leftCoverage - rightCoverage;
    }
    if (leftCoverage != null && rightCoverage == null) return -1;
    if (leftCoverage == null && rightCoverage != null) return 1;

    if (filter === "planning") {
      const leftRev = getConfirmedRevenue7d(left) ?? 0;
      const rightRev = getConfirmedRevenue7d(right) ?? 0;
      if (leftRev !== rightRev) return rightRev - leftRev;
    } else {
      const leftRevenueAtRisk = getConfirmedRevenueAtRisk7d(left);
      const rightRevenueAtRisk = getConfirmedRevenueAtRisk7d(right);
      if (
        leftRevenueAtRisk != null &&
        rightRevenueAtRisk != null &&
        leftRevenueAtRisk !== rightRevenueAtRisk
      ) {
        return rightRevenueAtRisk - leftRevenueAtRisk;
      }
      const leftRev = getConfirmedRevenue7d(left) ?? 0;
      const rightRev = getConfirmedRevenue7d(right) ?? 0;
      if (leftRev !== rightRev) return rightRev - leftRev;
    }

    const leftSku = left.sku ?? left.ozon_product_id;
    const rightSku = right.sku ?? right.ozon_product_id;
    if (leftSku !== rightSku) return leftSku - rightSku;
    return left.ozon_product_id - right.ozon_product_id;
  });
}

function parseOrderDeltaFromKeyMetric(keyMetric: string): number | null {
  const match = keyMetric.match(ORDER_DELTA_RE);
  if (!match) return null;
  const parsed = Number.parseInt(match[1], 10);
  return Number.isFinite(parsed) ? parsed : null;
}

function formatSignedCountDelta(value: number): string {
  if (value > 0) return `+${value}`;
  if (value < 0) return `−${Math.abs(value)}`;
  return "0";
}

function deltaTone(value: number): "default" | "negative" | "positive" {
  if (value < 0) return "negative";
  if (value > 0) return "positive";
  return "default";
}

function readNumericField(record: Record<string, unknown>, keys: readonly string[]): number | null {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "number" && Number.isFinite(value)) return value;
  }
  return null;
}

function getExplicitOrdersComparison(item: SKUFocusCard): { previous: number; current: number } | null {
  const record = item as SKUFocusCard & Record<string, unknown>;
  const previous = readNumericField(record, ORDERS_PREVIOUS_KEYS);
  const current = readNumericField(record, ORDERS_CURRENT_KEYS);
  if (previous == null || current == null) return null;
  return { previous, current };
}

function getRevenueValue(item: SKUFocusCard): string | null {
  const delta = getConfirmedRevenueDelta(item);
  if (delta != null) {
    return formatRevenueValue(delta);
  }

  const record = item as SKUFocusCard & Record<string, unknown>;
  const revenue = record.revenue;
  if (typeof revenue === "number" && Number.isFinite(revenue)) {
    return formatRevenueValue(revenue);
  }
  if (typeof revenue === "string" && revenue.trim().length > 0) {
    return revenue.trim();
  }
  return null;
}

function formatRevenueValue(value: number): string {
  const formatted = new Intl.NumberFormat("ru-RU", {
    maximumFractionDigits: 0,
  }).format(Math.abs(value));
  const sign = value > 0 ? "+" : value < 0 ? "−" : "";
  return `${sign}${formatted} ₽`;
}

/** Percent decline derived from explicit previous/current order counts (real numbers, not invented). */
function getOrdersDeclinePct(item: SKUFocusCard): number | null {
  const comparison = getExplicitOrdersComparison(item);
  if (!comparison || comparison.previous <= 0 || comparison.current >= comparison.previous) {
    return null;
  }
  return Math.round(((comparison.previous - comparison.current) / comparison.previous) * 1000) / 10;
}

function formatPctValue(value: number): string {
  return (
    new Intl.NumberFormat("ru-RU", {
      minimumFractionDigits: Number.isInteger(value) ? 0 : 1,
      maximumFractionDigits: 1,
    }).format(value) + "%"
  );
}

function getOrderDynamicsCandidate(item: SKUFocusCard): SkuFocusKpiCandidate {
  const delta = getConfirmedOrdersDelta(item);
  const declinePct = getOrdersDeclinePct(item);
  if (delta != null) {
    const deltaLabel = formatSignedCountDelta(delta);
    const value =
      declinePct != null ? `${deltaLabel} (−${formatPctValue(declinePct)})` : deltaLabel;
    return {
      key: "orders_delta",
      label: ORDERS_DYNAMICS_LABEL,
      sublabel: DAY_TO_DAY_SUBLABEL,
      value,
      tone: deltaTone(delta),
    };
  }

  return {
    key: "orders_delta",
    label: ORDERS_DYNAMICS_LABEL,
    sublabel: DAY_TO_DAY_SUBLABEL,
    value: null,
  };
}

function buildSkuFocusKpiCandidates(item: SKUFocusCard, tab: SKUFocusGroupKey): SkuFocusKpiCandidate[] {
  switch (tab) {
    case "order_drop":
      return buildOrderDropKpiCandidates(item);
    case "stock_risk":
      return buildStockRiskKpiCandidates(item);
    case "no_sales_with_stock":
      return buildNoSalesKpiCandidates(item);
    case "surplus":
      return buildStockKpiCandidates(item);
    default:
      return [];
  }
}

function buildKeySkuKpiCandidates(item: KeySkuCard): SkuFocusKpiCandidate[] {
  const share = formatKeySkuRevenueShare(item.revenue_share_pct);
  const revenue7d = formatKeySkuRevenue7d(item.revenue_7d);
  const coverage = formatKeySkuCoverageMetricValue(item);

  return [
    share
      ? {
          key: "revenue_share",
          label: KPI_LABELS.revenue_share,
          value: share.value,
          tone: share.tone,
        }
      : { key: "revenue_share", label: KPI_LABELS.revenue_share, value: null },
    revenue7d
      ? { key: "revenue_7d", label: KPI_LABELS.revenue_7d, value: revenue7d.value, tone: revenue7d.tone }
      : { key: "revenue_7d", label: KPI_LABELS.revenue_7d, value: null },
    coverage
      ? { key: "coverage", label: KPI_LABELS.coverage, value: coverage.value, tone: coverage.tone }
      : { key: "coverage", label: KPI_LABELS.coverage, value: null },
  ];
}

export function buildKeySkuKpiItems(item: KeySkuCard): SkuFocusKpiItem[] {
  return buildKeySkuKpiCandidates(item).flatMap((candidate) => {
    if (candidate.value == null) return [];
    return [
      {
        key: candidate.key,
        label: candidate.label ?? KPI_LABELS[candidate.key as SkuFocusKpiKey],
        sublabel: candidate.sublabel,
        value: candidate.value,
        tone: candidate.tone ?? "default",
      },
    ];
  });
}

function formatKeySkuRevenueShare(
  pct: number,
): { value: string; tone: "default" | "negative" | "positive" | "muted" } | null {
  if (!Number.isFinite(pct) || pct <= 0) return null;
  if (pct > 0 && pct < 1) {
    return { value: "<1%", tone: "default" };
  }
  return {
    value: new Intl.NumberFormat("ru-RU", {
      minimumFractionDigits: Number.isInteger(Math.round(pct * 10) / 10) ? 0 : 1,
      maximumFractionDigits: 1,
    }).format(Math.round(pct * 10) / 10) + "%",
    tone: "default",
  };
}

function formatKeySkuRevenue7d(
  value: number,
): { value: string; tone: "default" | "negative" | "positive" | "muted" } | null {
  if (!Number.isFinite(value) || value <= 0) return null;
  return { value: formatMoneyUnits(value), tone: "default" };
}

function formatKeySkuCoverageMetricValue(
  item: KeySkuCard,
): { value: string; tone: "default" | "negative" | "positive" | "muted" | "warning" | "attention" } | null {
  const coverage =
    item.coverage_days != null && Number.isFinite(item.coverage_days) && item.coverage_days > 0
      ? item.coverage_days
      : null;
  if (coverage == null) {
    return { value: "нет данных", tone: "muted" };
  }
  return { value: `${formatCoverageDays(coverage)} дн.`, tone: keySkuCoverageTone(coverage) };
}

function keySkuCoverageTone(days: number): "default" | "negative" | "positive" | "muted" | "warning" | "attention" {
  if (days <= 3) return "negative";
  if (days <= 7) return "warning";
  if (days <= 14) return "attention";
  return "positive";
}

function buildOrderDropKpiCandidates(item: SKUFocusCard): SkuFocusKpiCandidate[] {
  const ordersComparison = getExplicitOrdersComparison(item);
  const revenue = getRevenueValue(item);

  return [
    ordersComparison
      ? {
          key: "orders_comparison",
          label: KPI_LABELS.orders_comparison,
          value: `${ordersComparison.previous} → ${ordersComparison.current}`,
          tone: "default",
        }
      : { key: "orders_comparison", label: KPI_LABELS.orders_comparison, value: null },
    getOrderDynamicsCandidate(item),
    revenue
      ? {
          key: "revenue",
          label: KPI_LABELS.revenue,
          sublabel: DAY_TO_DAY_SUBLABEL,
          value: revenue,
          tone: deltaTone(getConfirmedRevenueDelta(item) ?? 0),
        }
      : { key: "revenue", label: KPI_LABELS.revenue, value: null },
  ];
}

function buildStockRiskKpiCandidates(item: SKUFocusCard): SkuFocusKpiCandidate[] {
  const stock = formatStockMetricValue(item);
  const coverage = formatCoverageMetricValue(item);
  const revenueAtRisk = formatRevenueAtRiskMetricValue(item);

  return [
    stock
      ? { key: "stock", label: KPI_LABELS.stock, value: stock.value, tone: stock.tone }
      : { key: "stock", label: KPI_LABELS.stock, value: null },
    coverage
      ? {
          key: "coverage",
          label: KPI_LABELS.coverage,
          value: coverage.value,
          tone: coverage.tone,
        }
      : { key: "coverage", label: KPI_LABELS.coverage, value: null },
    revenueAtRisk
      ? {
          key: "revenue_at_risk",
          label: KPI_LABELS.revenue_at_risk,
          sublabel: REVENUE_AT_RISK_SUBLABEL,
          value: revenueAtRisk.value,
          tone: revenueAtRisk.tone,
        }
      : { key: "revenue_at_risk", label: KPI_LABELS.revenue_at_risk, value: null },
  ];
}

function buildStockKpiCandidates(item: SKUFocusCard): SkuFocusKpiCandidate[] {
  const stock = parseMetricValue(item.key_metric, /Остаток\s+(\d+)/i);
  const revenue = getRevenueValue(item);

  return [
    stock
      ? { key: "stock", label: KPI_LABELS.stock, value: stock, tone: "default" }
      : { key: "stock", label: KPI_LABELS.stock, value: null },
    revenue
      ? { key: "revenue", label: KPI_LABELS.revenue, value: revenue, tone: "default" }
      : { key: "revenue", label: KPI_LABELS.revenue, value: null },
  ];
}

function buildNoSalesKpiCandidates(item: SKUFocusCard): SkuFocusKpiCandidate[] {
  const stock = formatNoSalesStockMetricValue(item);
  const noSalesDays = formatNoSalesDaysMetricValue(item);
  const frozenValue = formatFrozenStockMetricValue(item);

  return [
    stock
      ? { key: "stock", label: KPI_LABELS.stock, value: stock.value, tone: stock.tone }
      : { key: "stock", label: KPI_LABELS.stock, value: null },
    noSalesDays
      ? {
          key: "no_sales_days",
          label: KPI_LABELS.no_sales_days,
          value: noSalesDays.value,
          tone: noSalesDays.tone,
        }
      : { key: "no_sales_days", label: KPI_LABELS.no_sales_days, value: null },
    frozenValue
      ? {
          key: "frozen_stock_value",
          label: KPI_LABELS.frozen_stock_value,
          value: frozenValue.value,
          tone: frozenValue.tone,
        }
      : { key: "frozen_stock_value", label: KPI_LABELS.frozen_stock_value, value: null },
  ];
}

export function getVisibleSkuFocusKpiKeys(
  items: SKUFocusCard[],
  tab: SKUFocusGroupKey,
): SkuFocusKpiKey[] {
  const available = new Set<SkuFocusKpiKey>();
  for (const item of items) {
    for (const candidate of buildSkuFocusKpiCandidates(item, tab)) {
      if (candidate.value != null) {
        available.add(candidate.key);
      }
    }
  }
  return KPI_ORDER_BY_TAB[tab].filter((key) => available.has(key));
}

export function buildSkuFocusKpiItems(
  item: SKUFocusCard,
  tab: SKUFocusGroupKey,
  visibleKeys?: SkuFocusKpiKey[],
): SkuFocusKpiItem[] {
  const candidateMap = new Map(
    buildSkuFocusKpiCandidates(item, tab).map((candidate) => [candidate.key, candidate]),
  );
  const keys = visibleKeys ?? KPI_ORDER_BY_TAB[tab];

  return keys.flatMap((key) => {
    const candidate = candidateMap.get(key);
    if (!candidate || candidate.value == null) {
      return [];
    }
    return [
      {
        key,
        label: candidate.label ?? KPI_LABELS[key],
        sublabel: candidate.sublabel,
        value: candidate.value,
        tone: candidate.tone ?? "default",
      },
    ];
  });
}

export function hasSkuFocusProductImage(item: SKUFocusCard): boolean {
  return normalizeSkuFocusImageUrl(item) != null;
}

export function hasSkuProductDirectUrl(item: SKUFocusCard): boolean {
  return Boolean(item.product_url?.trim());
}

export function getSkuFocusProductSearchQuery(item: SKUFocusCard): string | null {
  const offerId = item.offer_id?.trim();
  if (offerId) return offerId;
  if (item.sku != null) return String(item.sku);
  return null;
}

export type SkuFocusCta = {
  href: string;
  label: string;
  kind: "direct" | "search";
};

export function resolveKeySkuCta(item: KeySkuCard): SkuFocusCta | null {
  const directUrl = item.product_url?.trim();
  if (directUrl) {
    return { href: directUrl, label: "Открыть товар", kind: "direct" };
  }
  // Internal /app/products search routes are retired from seller-facing CTAs.
  return null;
}

export function formatKeySkuLabel(item: KeySkuCard): string {
  if (item.sku != null) return `SKU ${item.sku}`;
  if (item.offer_id) return `Арт. ${item.offer_id}`;
  return "";
}

export function normalizeKeySkuImageUrl(item: KeySkuCard): string | null {
  if (item.image_url?.trim()) {
    return item.image_url.trim();
  }
  return null;
}

export function resolveSkuFocusCta(item: SKUFocusCard): SkuFocusCta | null {
  const directUrl = item.product_url?.trim();
  if (directUrl) {
    return { href: directUrl, label: "Открыть товар", kind: "direct" };
  }
  // Internal /app/products search routes are retired from seller-facing CTAs.
  return null;
}

function parseMetricValue(text: string, pattern: RegExp): string | null {
  const match = text.match(pattern);
  if (!match?.[1]) return null;
  const value = match[1].trim();
  return value.length > 0 ? value : null;
}

function parseMetricText(text: string, pattern: RegExp): string | null {
  const match = text.match(pattern);
  if (!match?.[1]) return null;
  const value = match[1].trim();
  return value.length > 0 ? value : null;
}

function formatCountUnits(value: number): string {
  return `${new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 }).format(value)} шт.`;
}

function formatDaysUnits(value: number): string {
  return `${new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 }).format(value)} дн.`;
}

function formatMoneyUnits(value: number): string {
  return `${new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 }).format(value)} ₽`;
}

function getConfirmedNoSalesDays(item: SKUFocusCard): number | null {
  if (item.no_sales_days != null && Number.isFinite(item.no_sales_days) && item.no_sales_days >= 1) {
    return item.no_sales_days;
  }
  return null;
}

function getConfirmedFrozenStockValue(item: SKUFocusCard): number | null {
  if (
    item.frozen_stock_value != null &&
    Number.isFinite(item.frozen_stock_value) &&
    item.frozen_stock_value > 0
  ) {
    return item.frozen_stock_value;
  }
  return null;
}

function formatNoSalesStockMetricValue(
  item: SKUFocusCard,
): { value: string; tone: "default" | "negative" | "positive" | "muted" } | null {
  const current =
    item.current_stock != null && Number.isFinite(item.current_stock)
      ? item.current_stock
      : readNumericField(item as SKUFocusCard & Record<string, unknown>, ["current_stock"]);
  if (current == null || current <= 0) {
    return null;
  }
  return { value: formatCountUnits(current), tone: "default" };
}

function formatNoSalesDaysMetricValue(
  item: SKUFocusCard,
): { value: string; tone: "default" | "negative" | "positive" | "muted" } | null {
  const days = getConfirmedNoSalesDays(item);
  if (days == null) {
    return null;
  }
  return { value: formatDaysUnits(days), tone: "default" };
}

function formatFrozenStockMetricValue(
  item: SKUFocusCard,
): { value: string; tone: "default" | "negative" | "positive" | "muted" } | null {
  const value = getConfirmedFrozenStockValue(item);
  if (value == null) {
    return null;
  }
  return { value: formatMoneyUnits(value), tone: "default" };
}

function formatCoverageDays(value: number): string {
  const formatted = new Intl.NumberFormat("ru-RU", {
    minimumFractionDigits: value < 10 ? 1 : 0,
    maximumFractionDigits: 1,
  }).format(value);
  return formatted;
}

function coverageTone(days: number): "default" | "negative" | "positive" | "muted" {
  if (days <= 3) return "negative";
  if (days <= 7) return "muted";
  return "default";
}

function formatStockMetricValue(
  item: SKUFocusCard,
): { value: string; tone: "default" | "negative" | "positive" | "muted" } | null {
  const current =
    item.current_stock != null && Number.isFinite(item.current_stock)
      ? item.current_stock
      : readNumericField(item as SKUFocusCard & Record<string, unknown>, ["current_stock"]);
  if (current == null) {
    const parsed = parseMetricValue(item.key_metric, /Остаток\s+(\d+)/i);
    if (!parsed) return null;
    return { value: formatCountUnits(Number.parseInt(parsed, 10)), tone: "default" };
  }
  const previous =
    item.previous_stock != null && Number.isFinite(item.previous_stock)
      ? item.previous_stock
      : readNumericField(item as SKUFocusCard & Record<string, unknown>, ["previous_stock"]);
  if (previous != null && previous !== current) {
    return { value: `${previous} → ${current} шт.`, tone: "default" };
  }
  return { value: formatCountUnits(current), tone: "default" };
}

function formatCoverageMetricValue(
  item: SKUFocusCard,
): { value: string; tone: "default" | "negative" | "positive" | "muted" } | null {
  const coverage = getConfirmedCoverageDays(item);
  if (coverage == null) return null;
  return { value: `${formatCoverageDays(coverage)} дн.`, tone: coverageTone(coverage) };
}

function formatRevenueAtRiskMetricValue(
  item: SKUFocusCard,
): { value: string; tone: "default" | "negative" | "positive" | "muted" } | null {
  const value = getConfirmedRevenueAtRisk7d(item);
  if (value == null) return null;
  const formatted = new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 }).format(
    Math.abs(value),
  );
  return { value: `${formatted} ₽ под риском`, tone: "negative" };
}

export function buildSkuFocusCheckText(item: SKUFocusCard, tab: SKUFocusGroupKey): string {
  const record = item as SKUFocusCard & Record<string, unknown>;
  for (const key of CHECK_HINT_KEYS) {
    const value = record[key];
    if (typeof value === "string" && value.trim().length > 0) {
      return value.trim();
    }
  }
  if (item.problem.trim().length > 0 && tab !== "order_drop") {
    return item.problem.trim();
  }
  return CHECK_FALLBACKS[tab];
}

export function getSkuProductHref(item: SKUFocusCard): string | null {
  return resolveSkuFocusCta(item)?.href ?? null;
}

export function formatSkuLabel(item: SKUFocusCard): string {
  if (item.sku != null) return `SKU ${item.sku}`;
  if (item.offer_id) return `Арт. ${item.offer_id}`;
  return "";
}
