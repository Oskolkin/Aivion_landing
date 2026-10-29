import { apiGet } from "@/lib/api";

export type ProductSparklinePoint = {
  date: string;
  /** Absent when daily_sku_metrics has no row for that day (not the same as zero sales). */
  orders_count?: number | null;
  revenue?: number | null;
};

export type ProductLastDayMetrics = {
  date: string;
  orders_count: number;
  units_sold: number;
  revenue: number;
};

export type ProductStockInfo = {
  stock_available: number;
  days_of_cover?: number | null;
  stock_status_bucket?: string | null;
};

export type ProductCatalogItem = {
  sku?: number | null;
  offer_id?: string | null;
  product_name: string;
  image_url?: string | null;
  current_price?: number | null;
  old_price?: number | null;
  min_price?: number | null;
  last_day?: ProductLastDayMetrics | null;
  sparkline_7d: ProductSparklinePoint[];
  stock?: ProductStockInfo | null;
  ozon_url?: string | null;
};

export type ProductsListResponse = {
  items: ProductCatalogItem[];
  total: number;
  limit: number;
  offset: number;
  latest_metric_date?: string | null;
};

export type ProductsQueryParams = {
  search?: string;
  limit?: number;
  offset?: number;
  sort?: string;
  has_sales_last_day?: boolean;
  stock_status?: string;
  filter?: string;
};

export function getProducts(params: ProductsQueryParams = {}): Promise<ProductsListResponse> {
  const query = new URLSearchParams();
  if (params.search?.trim()) query.set("search", params.search.trim());
  if (params.limit != null) query.set("limit", String(params.limit));
  if (params.offset != null) query.set("offset", String(params.offset));
  if (params.sort) query.set("sort", params.sort);
  if (params.has_sales_last_day === true) query.set("has_sales_last_day", "true");
  if (params.has_sales_last_day === false) query.set("has_sales_last_day", "false");
  if (params.stock_status) query.set("stock_status", params.stock_status);
  if (params.filter) query.set("filter", params.filter);
  const suffix = query.toString() ? `?${query.toString()}` : "";
  return apiGet<ProductsListResponse>(`/api/v1/products${suffix}`);
}
