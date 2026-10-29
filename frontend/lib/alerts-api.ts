import { apiGet, apiPost } from "@/lib/api";

export type AlertStatus = "open" | "resolved" | "dismissed";
/** API may return price_economics for history; seller UI treats it as legacy (see alert-labels.ts). */
export type AlertGroup = "sales" | "stock" | "advertising" | "price_economics";
export type AlertSeverity = "low" | "medium" | "high" | "critical";
export type AlertEntityType =
  | "account"
  | "sku"
  | "product"
  | "campaign"
  | "pricing_constraint";

export type AlertItem = {
  id: number;
  alert_type: string;
  alert_group: AlertGroup;
  entity_type: AlertEntityType;
  entity_id: string | null;
  entity_sku: number | null;
  entity_offer_id: string | null;
  title: string;
  message: string;
  severity: AlertSeverity;
  urgency: string;
  status: AlertStatus;
  evidence_payload: Record<string, unknown>;
  first_seen_at: string;
  last_seen_at: string;
  resolved_at: string | null;
  created_at: string;
  updated_at: string;
};

export type AlertsListResponse = {
  items: AlertItem[];
  limit: number;
  offset: number;
};

export type AlertsSummaryResponse = {
  open_total: number;
  critical_count: number;
  high_count: number;
  medium_count: number;
  low_count: number;
  by_group: {
    sales: number;
    stock: number;
    advertising: number;
    price_economics: number;
  };
  latest_run: {
    id: number;
    run_type: string;
    status: string;
    started_at: string;
    finished_at: string | null;
    sales_alerts_count: number;
    stock_alerts_count: number;
    ad_alerts_count: number;
    price_alerts_count: number;
    total_alerts_count: number;
    error_message: string | null;
  } | null;
};

export type RunAlertsResponse = {
  seller_account_id: number;
  as_of_date: string;
  run_id: number;
  status: string;
  sales: { generated_alerts: number; upserted_alerts: number; skipped_rules: number };
  stock: { generated_alerts: number; upserted_alerts: number; skipped_rules: number };
  advertising: { generated_alerts: number; upserted_alerts: number; skipped_rules: number };
  price_economics: { generated_alerts: number; upserted_alerts: number; skipped_rules: number };
  total_generated_alerts: number;
  total_upserted_alerts: number;
  total_skipped_rules: number;
  started_at: string;
  finished_at: string;
};

export type GetAlertsParams = {
  status?: AlertStatus;
  group?: AlertGroup;
  severity?: AlertSeverity;
  entityType?: AlertEntityType;
  limit?: number;
  offset?: number;
};

function buildQuery(params: Record<string, string | number | undefined>): string {
  const query = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === "") {
      continue;
    }
    query.set(k, String(v));
  }
  const encoded = query.toString();
  return encoded ? `?${encoded}` : "";
}

export async function getAlerts(params: GetAlertsParams): Promise<AlertsListResponse> {
  const query = buildQuery({
    status: params.status,
    group: params.group,
    severity: params.severity,
    entity_type: params.entityType,
    limit: params.limit,
    offset: params.offset,
  });
  return apiGet<AlertsListResponse>(`/api/v1/alerts${query}`);
}

export async function getAlertsSummary(): Promise<AlertsSummaryResponse> {
  return apiGet<AlertsSummaryResponse>("/api/v1/alerts/summary");
}

export async function runAlerts(payload?: {
  as_of_date?: string;
  run_type?: "manual" | "scheduled" | "post_sync" | "backfill";
}): Promise<RunAlertsResponse> {
  return apiPost<RunAlertsResponse>("/api/v1/alerts/run", payload);
}

export async function dismissAlert(id: number): Promise<AlertItem> {
  return apiPost<AlertItem>(`/api/v1/alerts/${id}/dismiss`);
}

export async function resolveAlert(id: number): Promise<AlertItem> {
  return apiPost<AlertItem>(`/api/v1/alerts/${id}/resolve`);
}
