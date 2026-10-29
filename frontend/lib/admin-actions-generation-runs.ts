export type AdminRunFilters = {
  seller_id?: number;
  status?: string;
  final_status?: string;
  metric_date?: string;
};

function cleanString(v: string | undefined): string | undefined {
  const out = v?.trim();
  return out ? out : undefined;
}

export function parseAdminRunFilters(params: Record<string, string | string[] | undefined>): AdminRunFilters {
  const first = (v: string | string[] | undefined): string | undefined => (Array.isArray(v) ? v[0] : v);
  const sellerRaw = cleanString(first(params.seller_id));
  const sellerNum = sellerRaw ? Number(sellerRaw) : NaN;
  return {
    seller_id: Number.isFinite(sellerNum) && sellerNum > 0 ? sellerNum : undefined,
    status: cleanString(first(params.status)),
    final_status: cleanString(first(params.final_status)),
    metric_date: cleanString(first(params.metric_date)),
  };
}

export function buildAdminRunFiltersQuery(filters: AdminRunFilters): string {
  const q = new URLSearchParams();
  if (filters.seller_id && filters.seller_id > 0) q.set("seller_id", String(filters.seller_id));
  if (cleanString(filters.status)) q.set("status", String(filters.status));
  if (cleanString(filters.final_status)) q.set("final_status", String(filters.final_status));
  if (cleanString(filters.metric_date)) q.set("metric_date", String(filters.metric_date));
  const encoded = q.toString();
  return encoded ? `?${encoded}` : "";
}
