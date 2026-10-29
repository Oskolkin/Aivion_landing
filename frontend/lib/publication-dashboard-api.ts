import { apiGet } from "@/lib/api";
import type { PublicationDashboardResponse, RowsResponse } from "@/lib/publication-dashboard";

export async function getPublicationDashboard(): Promise<PublicationDashboardResponse> {
  return apiGet<PublicationDashboardResponse>("/api/v1/dashboard/home");
}

export async function getStockSKUs(params: {
  publication_id: string;
  cursor?: string;
  coverage_band?: string;
  no_sales?: boolean;
  limit?: number;
}): Promise<RowsResponse> {
  const q = new URLSearchParams({ publication_id: params.publication_id });
  if (params.cursor) q.set("cursor", params.cursor);
  if (params.coverage_band) q.set("coverage_band", params.coverage_band);
  if (params.no_sales) q.set("no_sales", "true");
  if (params.limit) q.set("limit", String(params.limit));
  return apiGet<RowsResponse>(`/api/v1/dashboard/stock/skus?${q.toString()}`);
}

export async function getAdCampaigns(params: {
  publication_id: string;
  cursor?: string;
  limit?: number;
}): Promise<RowsResponse> {
  const q = new URLSearchParams({ publication_id: params.publication_id });
  if (params.cursor) q.set("cursor", params.cursor);
  if (params.limit) q.set("limit", String(params.limit));
  return apiGet<RowsResponse>(`/api/v1/dashboard/advertising/campaigns?${q.toString()}`);
}
