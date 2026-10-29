import { apiGet } from "@/lib/api";

export type AISummaryCard = {
  id?: string;
  summary_type: string;
  scope_type: string;
  scope_id?: string;
  scope_name?: string | null;
  status: string;
  headline?: string | null;
  health_score?: number | null;
  executive_summary?: string | null;
  summary_json?: Record<string, unknown>;
  updated_at?: string | null;
};

export type AISummariesLatestResponse = {
  status: string;
  message?: string;
  dashboard_metric_date?: string | null;
  metric_date?: string | null;
  run_status?: string | null;
  failed_category_count?: number | null;
  generated_at?: string | null;
  account_summary?: AISummaryCard | null;
  category_summaries: AISummaryCard[];
};

export async function getLatestAISummaries(): Promise<AISummariesLatestResponse> {
  return apiGet<AISummariesLatestResponse>("/api/v1/ai-summaries/latest");
}
