import { apiGet } from "@/lib/api";

export type DashboardDelta = {
  abs: number;
  pct: number | null;
};

export type DashboardSummaryResponse = {
  kpi: {
    revenue_current: number;
    revenue_day_to_day_delta: DashboardDelta;
    revenue_week_to_week_delta: DashboardDelta;
    orders_current: number;
    orders_day_to_day_delta: number;
    returns_current: number;
    cancels_current: number;
  };
  summary: {
    last_successful_update: string | null;
    period_used: string;
    data_freshness: string;
    as_of_date: string;
    as_of_date_source: string;
    kpi_semantics: string;
    sku_orders_semantics: string;
  };
  top_skus: DashboardSkuRow[];
};

export type DashboardSkuRow = {
  ozon_product_id: number;
  offer_id: string | null;
  sku: number | null;
  product_name: string | null;
  revenue: number;
  orders_count: number;
  share_of_revenue: number | null;
  contribution_to_revenue_change: number;
  stock_available: number;
  days_of_cover: number | null;
};

export type DashboardSkuTableResponse = {
  items: DashboardSkuRow[];
  total: number;
  limit: number;
  offset: number;
};

export type DashboardStockRow = {
  ozon_product_id: number;
  offer_id: string | null;
  sku: number | null;
  product_name: string | null;
  warehouse: string;
  quantity_total: number;
  quantity_reserved: number;
  quantity_available: number;
  snapshot_at: string | null;
};

export type DashboardStocksResponse = {
  items: DashboardStockRow[];
  total: number;
};

export type SKUQueryParams = {
  asOfDate?: string;
  limit?: number;
  offset?: number;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
};

export type CriticalSKUItem = {
  ozon_product_id: number;
  offer_id: string | null;
  sku: number | null;
  product_name: string | null;
  revenue: number;
  sales_ops: number;
  revenue_delta_day: number;
  orders_delta_day: number;
  stock_available: number;
  days_of_cover: number | null;
  importance: number;
  out_of_stock_risk: number;
  problem_score: number;
  signals: string[] | null;
  badges: string[] | null;
};

export type CriticalSKUsResponse = {
  items: CriticalSKUItem[];
  meta: {
    as_of_date: string;
    scoring_semantics: Record<string, string>;
    latest_data_timestamp: string | null;
    total: number;
    limit: number;
    offset: number;
    sort_by: string;
    sort_order: "asc" | "desc";
  };
};

export type CriticalSKUsQueryParams = {
  asOfDate?: string;
  limit?: number;
  offset?: number;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
};

export type StocksReplenishmentItem = {
  ozon_product_id: number;
  offer_id: string | null;
  sku: number | null;
  product_name: string | null;
  current_total_stock: number;
  current_reserved_stock: number;
  current_available_stock: number;
  days_of_cover: number | null;
  snapshot_at: string | null;
  warehouse_count: number;
  importance: number;
  depletion_risk: string;
  replenishment_priority: string;
  signals: string[] | null;
};

export type StocksReplenishmentResponse = {
  items: StocksReplenishmentItem[];
  meta: {
    stock_semantics: string;
    as_of_date: string;
    last_stock_update: string | null;
  };
};

export type AdvertisingAnalyticsResponse = {
  summary?: Record<string, unknown> | null;
  items?: Array<Record<string, unknown>> | null;
  campaigns?: Array<Record<string, unknown>> | null;
  risks?: Array<Record<string, unknown>> | null;
  sku_risks?: Array<Record<string, unknown>> | null;
  [key: string]: unknown;
};

function buildQuery(params: Record<string, string | number | undefined>): string {
  const searchParams = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === "") {
      return;
    }
    searchParams.set(key, String(value));
  });
  const query = searchParams.toString();
  return query ? `?${query}` : "";
}

export async function getDashboardSummary(
  asOfDate?: string
): Promise<DashboardSummaryResponse> {
  const query = buildQuery({ as_of_date: asOfDate });
  return apiGet<DashboardSummaryResponse>(`/api/v1/analytics/dashboard${query}`);
}

export async function getDashboardSKUTable(
  params: SKUQueryParams
): Promise<DashboardSkuTableResponse> {
  const query = buildQuery({
    as_of_date: params.asOfDate,
    limit: params.limit,
    offset: params.offset,
    sort_by: params.sortBy,
    sort_order: params.sortOrder,
  });
  return apiGet<DashboardSkuTableResponse>(`/api/v1/analytics/sku-table${query}`);
}

export async function getDashboardStocks(): Promise<DashboardStocksResponse> {
  return apiGet<DashboardStocksResponse>("/api/v1/analytics/stocks");
}

export async function getCriticalSKUs(
  params: CriticalSKUsQueryParams
): Promise<CriticalSKUsResponse> {
  const query = buildQuery({
    as_of_date: params.asOfDate,
    limit: params.limit,
    offset: params.offset,
    sort_by: params.sortBy,
    sort_order: params.sortOrder,
  });
  return apiGet<CriticalSKUsResponse>(`/api/v1/analytics/critical-skus${query}`);
}

export async function getStocksReplenishment(
  asOfDate?: string
): Promise<StocksReplenishmentResponse> {
  const query = buildQuery({ as_of_date: asOfDate });
  return apiGet<StocksReplenishmentResponse>(
    `/api/v1/analytics/stocks-replenishment${query}`
  );
}

export async function getAdvertisingAnalytics(): Promise<AdvertisingAnalyticsResponse> {
  return apiGet<AdvertisingAnalyticsResponse>("/api/v1/analytics/advertising");
}
