import { apiGet, apiPost } from "@/lib/api";
import { ACTIVE_RECOMMENDATION_TYPES } from "@/lib/recommendation-labels";

/** Active MVP recommendation types for seller filters (price/economics excluded). */
export const MVP_RECOMMENDATION_TYPES = [...ACTIVE_RECOMMENDATION_TYPES] as const;

export type RecommendationStatus = "open" | "accepted" | "dismissed" | "resolved";
export type RecommendationPriorityLevel = "low" | "medium" | "high" | "critical";
export type RecommendationConfidenceLevel = "low" | "medium" | "high";
export type RecommendationHorizon = "short_term" | "medium_term" | "long_term";

/** Aliases for API contract / UI layer naming */
export type PriorityLevel = RecommendationPriorityLevel;
export type ConfidenceLevel = RecommendationConfidenceLevel;
export type RecommendationEntityType =
  | "account"
  | "sku"
  | "product"
  | "campaign"
  | "pricing_constraint";

export type RelatedAlert = {
  id: number;
  alert_type: string;
  alert_group: string;
  entity_type: string;
  entity_id: string | null;
  entity_sku: number | null;
  entity_offer_id: string | null;
  title: string;
  message: string;
  severity: string;
  urgency: string;
  status: string;
  evidence_payload: Record<string, unknown>;
  first_seen_at: string;
  last_seen_at: string;
};

export type RecommendationItem = {
  id: number;
  source: string;
  recommendation_type: string;
  horizon: string;
  entity_type: string;
  entity_id: string | null;
  entity_sku: number | null;
  entity_offer_id: string | null;
  title: string;
  what_happened: string;
  why_it_matters: string;
  recommended_action: string;
  expected_effect: string | null;
  priority_score: number;
  priority_level: string;
  urgency: string;
  confidence_level: string;
  status: string;
  supporting_metrics_payload: Record<string, unknown>;
  constraints_payload: Record<string, unknown>;
  ai_model: string | null;
  ai_prompt_version: string | null;
  first_seen_at: string;
  last_seen_at: string;
  accepted_at: string | null;
  dismissed_at: string | null;
  resolved_at: string | null;
  created_at: string;
  updated_at: string;
  related_alerts?: RelatedAlert[];
};

export type RecommendationDetail = RecommendationItem & {
  related_alerts?: RelatedAlert[];
  /** If API adds per-item validation warnings */
  validation_warnings?: string[];
};

export type RecommendationRunSummary = {
  id: number;
  run_type: string;
  status: string;
  started_at: string;
  finished_at: string | null;
  as_of_date: string | null;
  ai_model: string | null;
  ai_prompt_version: string | null;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  estimated_cost: number;
  generated_recommendations_count: number;
  error_message: string | null;
};

export type RecommendationsSummary = {
  open_total: number;
  by_priority: {
    low: number;
    medium: number;
    high: number;
    critical: number;
  };
  by_confidence: {
    low: number;
    medium: number;
    high: number;
  };
  latest_run: RecommendationRunSummary | null;
};

/** @deprecated Use RecommendationsSummary */
export type RecommendationSummary = RecommendationsSummary;

export type RecommendationFilters = {
  status?: string;
  recommendation_type?: string;
  priority_level?: string;
  confidence_level?: string;
  horizon?: string;
  entity_type?: string;
  limit?: number;
  offset?: number;
};

/** @deprecated Use RecommendationFilters */
export type RecommendationsListParams = RecommendationFilters;

export type RecommendationsListResponse = {
  items: RecommendationItem[];
  limit: number;
  offset: number;
};

export type GenerateRecommendationsRequest = {
  as_of_date?: string;
  run_type?: "manual" | "scheduled" | "post_alerts" | "backfill";
};

export type GenerateRecommendationsResponse = {
  seller_account_id: number;
  as_of_date: string;
  run_id: number;
  generated_total: number;
  valid_total: number;
  rejected_total: number;
  upserted_total: number;
  linked_alerts_total: number;
  warnings_total: number;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  /** Present when API returns cost fields (optional for older backends). */
  estimated_cost?: number;
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

export async function getRecommendations(params: RecommendationFilters): Promise<RecommendationsListResponse> {
  const query = buildQuery({
    status: params.status,
    recommendation_type: params.recommendation_type,
    priority_level: params.priority_level,
    confidence_level: params.confidence_level,
    horizon: params.horizon,
    entity_type: params.entity_type,
    limit: params.limit,
    offset: params.offset,
  });
  return apiGet<RecommendationsListResponse>(`/api/v1/recommendations${query}`);
}

export async function getRecommendationsSummary(): Promise<RecommendationsSummary> {
  return apiGet<RecommendationsSummary>("/api/v1/recommendations/summary");
}

export async function getRecommendationDetail(id: number): Promise<RecommendationDetail> {
  return apiGet<RecommendationDetail>(`/api/v1/recommendations/${id}`);
}

export async function generateRecommendations(
  payload?: GenerateRecommendationsRequest,
): Promise<GenerateRecommendationsResponse> {
  return apiPost<GenerateRecommendationsResponse>("/api/v1/recommendations/generate", payload ?? {});
}

export async function acceptRecommendation(id: number): Promise<RecommendationItem> {
  return apiPost<RecommendationItem>(`/api/v1/recommendations/${id}/accept`);
}

export async function dismissRecommendation(id: number): Promise<RecommendationItem> {
  return apiPost<RecommendationItem>(`/api/v1/recommendations/${id}/dismiss`);
}

export async function resolveRecommendation(id: number): Promise<RecommendationItem> {
  return apiPost<RecommendationItem>(`/api/v1/recommendations/${id}/resolve`);
}
