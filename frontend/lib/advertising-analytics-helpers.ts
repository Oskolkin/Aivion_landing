import type { AdvertisingAnalyticsResponse } from "@/lib/analytics-api";

export type ParsedAdRiskRow = {
  title: string;
  campaignLabel: string;
  entityLabel: string;
  spend: number;
  revenue: number;
  orders: number;
  roas: number | null;
  reason: string;
  lowStockFlag: boolean;
};

/** True when ads API error likely indicates Performance API connection/credentials problem. */
export function isLikelyAdsPerformanceConnectionIssue(message: string): boolean {
  const m = message.toLowerCase();
  return (
    m.includes("performance") ||
    m.includes("unauthorized") ||
    m.includes("forbidden") ||
    m.includes("401") ||
    m.includes("403") ||
    m.includes("connection_required")
  );
}

/** @deprecated Use isLikelyAdsPerformanceConnectionIssue */
export const isLikelyAdsPerformanceTokenIssue = isLikelyAdsPerformanceConnectionIssue;

function toRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function toNum(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return null;
}

function toStr(value: unknown): string | null {
  return typeof value === "string" && value.trim() ? value : null;
}

function getNum(obj: Record<string, unknown>, keys: string[]): number | null {
  for (const key of keys) {
    const value = toNum(obj[key]);
    if (value != null) return value;
  }
  return null;
}

function getStr(obj: Record<string, unknown>, keys: string[]): string | null {
  for (const key of keys) {
    const value = toStr(obj[key]);
    if (value) return value;
  }
  return null;
}

function getArray(obj: Record<string, unknown>, keys: string[]): Array<Record<string, unknown>> {
  for (const key of keys) {
    const raw = obj[key];
    if (!Array.isArray(raw)) continue;
    return raw
      .map((entry) => toRecord(entry))
      .filter((entry): entry is Record<string, unknown> => entry != null);
  }
  return [];
}

function computeRoas(row: Record<string, unknown>, revenue: number, spend: number): number | null {
  const existing = getNum(row, ["roas", "roas_value", "roas_ratio"]);
  if (existing != null) return existing;
  if (spend <= 0) return null;
  return revenue / spend;
}

function formatAdRiskReason(
  spend: number,
  revenue: number,
  orders: number,
  roas: number | null,
  lowStockFlag: boolean,
): string {
  if (spend > 0 && revenue <= 0) return "Spend without result";
  if (spend > 0 && orders <= 0) return "No orders";
  if (roas != null && roas < 1) return "Weak ROAS";
  if (lowStockFlag) return "Low-stock advertised SKU";
  if (spend > 0) return "High spend";
  return "Advertising risk";
}

function formatEntityLabel(row: Record<string, unknown>): string {
  const skuStr = getStr(row, ["sku", "entity_sku"]);
  const skuNum = getNum(row, ["sku", "entity_sku"]);
  const offer = getStr(row, ["offer_id", "entity_offer_id"]);
  const productId = getNum(row, ["product_id", "ozon_product_id"]);
  if (skuStr) return `SKU: ${skuStr}`;
  if (skuNum != null) return `SKU: ${String(Math.trunc(skuNum))}`;
  if (offer) return `Offer: ${offer}`;
  if (productId != null) return `Product: ${String(Math.trunc(productId))}`;
  return "—";
}

export function toAdRiskRow(row: Record<string, unknown>): ParsedAdRiskRow {
  const spend =
    getNum(row, ["spend_total", "spend", "total_spend", "ad_spend", "cost"]) ?? 0;
  const revenue =
    getNum(row, ["revenue_total", "revenue", "attributed_revenue", "sales_revenue"]) ?? 0;
  const orders = getNum(row, ["orders_total", "orders", "orders_count", "attributed_orders"]) ?? 0;
  const roas = computeRoas(row, revenue, spend);
  const daysOfCover = getNum(row, ["days_of_cover", "stock_days_of_cover"]);
  const lowStockFlag =
    row.low_stock === true ||
    row.low_stock_flag === true ||
    row.stock_risk === true ||
    (typeof row.stock_signal === "string" &&
      row.stock_signal.toLowerCase().includes("low")) ||
    (daysOfCover != null && daysOfCover <= 3);
  const title =
    getStr(row, ["campaign_name", "name", "title", "product_name", "sku_name"]) ?? "Advertising entity";
  const campaignName = getStr(row, ["campaign_name", "campaign_title", "campaign"]);
  const campaignIdStr = getStr(row, ["campaign_id", "external_campaign_id", "id"]);
  const campaignIdNum = getNum(row, ["campaign_external_id"]);
  const campaignId =
    campaignIdStr ?? (campaignIdNum != null ? String(Math.trunc(campaignIdNum)) : null);
  const campaignLabel = campaignName
    ? campaignName
    : campaignId
      ? "Кампания без названия"
      : "Campaign: not available";

  const explicitReason = getStr(row, ["combined_reason", "risk_reason", "reason", "efficiency_signal"]);
  const reason =
    explicitReason && explicitReason.length > 2
      ? explicitReason
      : formatAdRiskReason(spend, revenue, orders, roas, lowStockFlag);

  return {
    title,
    campaignLabel,
    entityLabel: formatEntityLabel(row),
    spend,
    revenue,
    orders,
    roas,
    lowStockFlag,
    reason,
  };
}

function collectCandidateRows(response: AdvertisingAnalyticsResponse): Record<string, unknown>[] {
  const root = toRecord(response) ?? {};
  const summary = toRecord(root.summary ?? null);
  return [
    ...getArray(root, ["items", "campaigns", "risks", "sku_risks", "top_risks"]),
    ...getArray(summary ?? {}, ["top_risks", "campaigns", "items", "risks", "sku_risks"]),
  ];
}

export function collectAdRiskRows(
  response: AdvertisingAnalyticsResponse,
  limit = 200,
): ParsedAdRiskRow[] {
  const rows = collectCandidateRows(response)
    .map(toAdRiskRow)
    .filter((row) => row.spend > 0 || row.orders > 0 || row.revenue > 0)
    .sort((a, b) => {
      const aSeverity = a.spend > 0 && (a.orders <= 0 || a.revenue <= 0) ? 0 : a.roas != null && a.roas < 1 ? 1 : 2;
      const bSeverity = b.spend > 0 && (b.orders <= 0 || b.revenue <= 0) ? 0 : b.roas != null && b.roas < 1 ? 1 : 2;
      if (aSeverity !== bSeverity) return aSeverity - bSeverity;
      if (a.spend !== b.spend) return b.spend - a.spend;
      if (a.orders !== b.orders) return a.orders - b.orders;
      return a.revenue - b.revenue;
    });
  return rows.slice(0, limit);
}

export function summarizeAdRisksFromResponse(
  response: AdvertisingAnalyticsResponse,
  rows: ParsedAdRiskRow[],
) {
  const root = toRecord(response) ?? {};
  const summary = toRecord(root.summary ?? null) ?? {};
  const totalSpend =
    getNum(summary, ["total_spend", "spend", "ad_spend"]) ?? rows.reduce((acc, row) => acc + row.spend, 0);
  const weakCampaigns =
    getNum(summary, ["weak_campaigns_count", "weak_campaigns", "low_efficiency_campaigns_count"]) ??
    rows.filter((row) => row.roas != null && row.roas < 1).length;
  const spendWithoutResult =
    getNum(summary, ["spend_without_result_count", "zero_result_campaigns_count"]) ??
    rows.filter((row) => row.spend > 0 && (row.orders <= 0 || row.revenue <= 0)).length;
  const lowStockAdvertisedSkus =
    getNum(summary, ["low_stock_advertised_skus_count", "low_stock_skus_count"]) ??
    rows.filter((row) => row.lowStockFlag).length;

  const campaignsFromSummary =
    getNum(summary, ["active_campaigns_count", "campaigns_count", "total_campaigns"]) ?? null;
  const rawCampaigns = getArray(root, ["campaigns"]);
  const campaignsCount =
    campaignsFromSummary != null ? campaignsFromSummary : rawCampaigns.length > 0 ? rawCampaigns.length : null;

  return { totalSpend, weakCampaigns, spendWithoutResult, lowStockAdvertisedSkus, campaignsCount };
}
