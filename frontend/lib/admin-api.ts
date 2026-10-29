import { apiGet, apiPut } from "@/lib/api";

import { PUBLIC_API_BASE_URL } from "@/lib/env/api-base-url";

function buildQuery(params: Record<string, string | number | undefined>): string {
  const query = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === "") continue;
    query.set(k, String(v));
  }
  const encoded = query.toString();
  return encoded ? `?${encoded}` : "";
}

export type AdminMeResponse = {
  is_admin: boolean;
  email: string;
};

export async function getAdminMe(): Promise<AdminMeResponse> {
  return apiGet<AdminMeResponse>("/api/v1/admin/me");
}

export type AdminClientListItem = {
  seller_account_id: number;
  seller_name: string;
  user_email: string;
  seller_status: string;
  connection_status?: string | null;
  last_check_at?: string | null;
  latest_sync_status?: string | null;
  latest_sync_started_at?: string | null;
  latest_sync_finished_at?: string | null;
  open_alerts_count: number;
  open_recommendations_count: number;
  latest_recommendation_run_status?: string | null;
  latest_chat_trace_status?: string | null;
  billing_status?: string | null;
  created_at: string;
  updated_at: string;
};

export type AdminClientsResponse = {
  items: AdminClientListItem[];
  limit: number;
  offset: number;
};

export type AdminClientsParams = {
  search?: string;
  status?: string;
  connection_status?: string;
  limit?: number;
  offset?: number;
};

export async function getAdminClients(params?: AdminClientsParams): Promise<AdminClientsResponse> {
  const query = buildQuery({
    search: params?.search,
    status: params?.status,
    connection_status: params?.connection_status,
    limit: params?.limit,
    offset: params?.offset,
  });
  return apiGet<AdminClientsResponse>(`/api/v1/admin/clients${query}`);
}

export type AdminClientDetail = {
  overview: {
    seller_account_id: number;
    seller_name: string;
    seller_status: string;
    owner_user_id?: number | null;
    owner_email?: string | null;
    created_at: string;
    updated_at: string;
  };
  connections: Array<{
    provider: string;
    status: string;
    last_check_at?: string | null;
    last_check_result?: string | null;
    last_error?: string | null;
    updated_at?: string | null;
    performance_connection_status: string;
    performance_token_set: boolean;
    performance_last_check_at?: string | null;
    performance_last_check_result?: string | null;
    performance_last_error?: string | null;
  }>;
  operational_status: {
    latest_sync_job?: AdminSyncJobItem | null;
    latest_import_jobs: AdminImportJobItem[];
    latest_alert_run?: {
      id: number;
      run_type: string;
      status: string;
      started_at?: string | null;
      finished_at?: string | null;
      total_alerts_count: number;
      error_message?: string | null;
    } | null;
    latest_recommendation_run?: {
      id: number;
      run_type: string;
      status: string;
      started_at?: string | null;
      finished_at?: string | null;
      input_tokens: number;
      output_tokens: number;
      generated_recommendations_count: number;
      accepted_recommendations_count: number;
      error_message?: string | null;
    } | null;
    latest_chat_trace?: {
      id: number;
      session_id: number;
      status: string;
      detected_intent?: string | null;
      started_at?: string | null;
      finished_at?: string | null;
      error_message?: string | null;
    } | null;
    open_alerts_count: number;
    open_recommendations_count: number;
    limitations: string[];
  };
  billing?: AdminBillingState | null;
};

export async function getAdminClientDetail(sellerAccountId: number): Promise<AdminClientDetail> {
  return apiGet<AdminClientDetail>(`/api/v1/admin/clients/${sellerAccountId}`);
}

export type AdminPaged<T> = { items: T[]; limit: number; offset: number };

export type AdminSyncJobItem = {
  id: number;
  type: string;
  status: string;
  started_at?: string | null;
  finished_at?: string | null;
  error_message?: string | null;
  created_at?: string | null;
};

export type AdminImportJobItem = {
  id: number;
  sync_job_id: number;
  domain: string;
  status: string;
  source_cursor?: string | null;
  records_received: number;
  records_imported: number;
  records_failed: number;
  started_at?: string | null;
  finished_at?: string | null;
  error_message?: string | null;
  created_at?: string | null;
};

export type AdminImportErrorItem = {
  import_job_id: number;
  sync_job_id: number;
  domain: string;
  status: string;
  error_message: string;
  records_failed: number;
  started_at?: string | null;
  finished_at?: string | null;
};

export type AdminSyncCursorItem = {
  domain: string;
  cursor_type: string;
  cursor_value?: string | null;
  updated_at: string;
};

export async function getAdminSyncJobs(
  sellerAccountId: number,
  params?: { status?: string; limit?: number; offset?: number },
): Promise<AdminPaged<AdminSyncJobItem>> {
  const query = buildQuery({ status: params?.status, limit: params?.limit, offset: params?.offset });
  return apiGet<AdminPaged<AdminSyncJobItem>>(`/api/v1/admin/clients/${sellerAccountId}/sync-jobs${query}`);
}

export async function getAdminImportJobs(
  sellerAccountId: number,
  params?: { status?: string; domain?: string; limit?: number; offset?: number },
): Promise<AdminPaged<AdminImportJobItem>> {
  const query = buildQuery({
    status: params?.status,
    domain: params?.domain,
    limit: params?.limit,
    offset: params?.offset,
  });
  return apiGet<AdminPaged<AdminImportJobItem>>(`/api/v1/admin/clients/${sellerAccountId}/import-jobs${query}`);
}

export async function getAdminImportErrors(
  sellerAccountId: number,
  params?: { status?: string; domain?: string; limit?: number; offset?: number },
): Promise<AdminPaged<AdminImportErrorItem>> {
  const query = buildQuery({
    status: params?.status,
    domain: params?.domain,
    limit: params?.limit,
    offset: params?.offset,
  });
  return apiGet<AdminPaged<AdminImportErrorItem>>(`/api/v1/admin/clients/${sellerAccountId}/import-errors${query}`);
}

export async function getAdminSyncCursors(
  sellerAccountId: number,
  params?: { domain?: string; limit?: number; offset?: number },
): Promise<AdminPaged<AdminSyncCursorItem>> {
  const query = buildQuery({ domain: params?.domain, limit: params?.limit, offset: params?.offset });
  return apiGet<AdminPaged<AdminSyncCursorItem>>(`/api/v1/admin/clients/${sellerAccountId}/sync-cursors${query}`);
}

export type AdminRecommendationRunItem = {
  id: number;
  run_type: string;
  status: string;
  as_of_date?: string | null;
  ai_model?: string | null;
  ai_prompt_version?: string | null;
  input_tokens: number;
  output_tokens: number;
  estimated_cost: number;
  generated_recommendations_count: number;
  accepted_recommendations_count: number;
  rejected_recommendations_count?: number | null;
  error_message?: string | null;
  started_at?: string | null;
  finished_at?: string | null;
};

export type AdminRecommendationItem = {
  id: number;
  recommendation_type: string;
  title: string;
  status: string;
  priority_level: string;
  confidence_level: string;
  horizon: string;
  entity_type: string;
  entity_id?: string | null;
  entity_sku?: number | null;
  entity_offer_id?: string | null;
  what_happened?: string | null;
  why_it_matters?: string | null;
  recommended_action?: string | null;
  expected_effect?: string | null;
  supporting_metrics_payload?: Record<string, unknown>;
  constraints_payload?: Record<string, unknown>;
  raw_ai_response?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type AdminRecommendationDiagnosticItem = {
  id: number;
  openai_request_id?: string | null;
  ai_model?: string | null;
  prompt_version?: string | null;
  context_payload_summary?: Record<string, unknown>;
  raw_openai_response?: Record<string, unknown>;
  validation_result_payload?: Record<string, unknown>;
  rejected_items_payload?: Record<string, unknown>;
  error_stage?: string | null;
  error_message?: string | null;
  input_tokens: number;
  output_tokens: number;
  estimated_cost: number;
  created_at: string;
};

export type AdminRecommendationRunDetail = {
  run: AdminRecommendationRunItem;
  recommendations: AdminRecommendationItem[];
  diagnostics: AdminRecommendationDiagnosticItem[];
  limitations: string[];
};

export type AdminRecommendationRelatedAlertBrief = {
  id: number;
  alert_type: string;
  alert_group: string;
  severity: string;
  urgency: string;
  title: string;
  status: string;
};

export type AdminRecommendationRawAIDetail = {
  recommendation: AdminRecommendationItem;
  related_alerts: AdminRecommendationRelatedAlertBrief[];
  diagnostics: AdminRecommendationDiagnosticItem[];
  limitations: string[];
};

export async function getAdminRecommendationRuns(
  sellerAccountId: number,
  params?: { status?: string; run_type?: string; limit?: number; offset?: number },
): Promise<AdminPaged<AdminRecommendationRunItem>> {
  const query = buildQuery({
    status: params?.status,
    run_type: params?.run_type,
    limit: params?.limit,
    offset: params?.offset,
  });
  return apiGet<AdminPaged<AdminRecommendationRunItem>>(
    `/api/v1/admin/clients/${sellerAccountId}/ai/recommendation-runs${query}`,
  );
}

export async function getAdminRecommendationRunDetail(
  sellerAccountId: number,
  runId: number,
): Promise<AdminRecommendationRunDetail> {
  return apiGet<AdminRecommendationRunDetail>(`/api/v1/admin/clients/${sellerAccountId}/ai/recommendation-runs/${runId}`);
}

export async function getAdminRecommendationDetail(
  sellerAccountId: number,
  recommendationId: number,
): Promise<AdminRecommendationRawAIDetail> {
  return apiGet<AdminRecommendationRawAIDetail>(`/api/v1/admin/clients/${sellerAccountId}/ai/recommendations/${recommendationId}`);
}

export type AdminChatTraceItem = {
  id: number;
  session_id: number;
  user_message_id?: number | null;
  assistant_message_id?: number | null;
  detected_intent?: string | null;
  status: string;
  planner_model: string;
  answer_model: string;
  planner_prompt_version: string;
  answer_prompt_version: string;
  input_tokens: number;
  output_tokens: number;
  estimated_cost: number;
  error_message?: string | null;
  started_at?: string | null;
  finished_at?: string | null;
  created_at?: string | null;
};

export type AdminChatTracePayloads = {
  tool_plan_payload?: Record<string, unknown>;
  validated_tool_plan_payload?: Record<string, unknown>;
  tool_results_payload?: Record<string, unknown>;
  fact_context_payload?: Record<string, unknown>;
  raw_planner_response?: Record<string, unknown>;
  raw_answer_response?: Record<string, unknown>;
  answer_validation_payload?: Record<string, unknown>;
};

export type AdminChatTraceDetail = {
  trace: AdminChatTraceItem;
  messages: AdminChatMessageItem[];
  payloads: AdminChatTracePayloads;
  limitations: string[];
};

export async function getAdminChatTraces(
  sellerAccountId: number,
  params?: { status?: string; intent?: string; session_id?: number; limit?: number; offset?: number },
): Promise<AdminPaged<AdminChatTraceItem>> {
  const query = buildQuery({
    status: params?.status,
    intent: params?.intent,
    session_id: params?.session_id,
    limit: params?.limit,
    offset: params?.offset,
  });
  return apiGet<AdminPaged<AdminChatTraceItem>>(`/api/v1/admin/clients/${sellerAccountId}/ai/chat-traces${query}`);
}

export async function getAdminChatTraceDetail(
  sellerAccountId: number,
  traceId: number,
): Promise<AdminChatTraceDetail> {
  return apiGet<AdminChatTraceDetail>(`/api/v1/admin/clients/${sellerAccountId}/ai/chat-traces/${traceId}`);
}

export type AdminChatSessionItem = {
  id: number;
  title: string;
  status: string;
  created_at: string;
  updated_at: string;
  last_message_at?: string | null;
};

export async function getAdminChatSessions(
  sellerAccountId: number,
  params?: { status?: string; limit?: number; offset?: number },
): Promise<AdminPaged<AdminChatSessionItem>> {
  const query = buildQuery({ status: params?.status, limit: params?.limit, offset: params?.offset });
  return apiGet<AdminPaged<AdminChatSessionItem>>(`/api/v1/admin/clients/${sellerAccountId}/chat/sessions${query}`);
}

export type AdminChatMessageItem = {
  id: number;
  session_id: number;
  role: string;
  message_type: string;
  content: string;
  created_at: string;
};

export async function getAdminChatMessages(
  sellerAccountId: number,
  sessionId: number,
  params?: { limit?: number; offset?: number },
): Promise<AdminPaged<AdminChatMessageItem>> {
  const query = buildQuery({ limit: params?.limit, offset: params?.offset });
  return apiGet<AdminPaged<AdminChatMessageItem>>(
    `/api/v1/admin/clients/${sellerAccountId}/chat/sessions/${sessionId}/messages${query}`,
  );
}

export type AdminClientChatFeedbackItem = {
  id: number;
  seller_account_id: number;
  seller_name?: string | null;
  session_id: number;
  message_id: number;
  rating: string;
  comment?: string | null;
  created_at: string;
  trace_id?: number | null;
  message?: { id: number; role?: string | null; message_type?: string | null; content?: string | null };
  session?: { id: number; title?: string | null };
};

export async function getAdminClientChatFeedback(
  sellerAccountId: number,
  params?: { rating?: string; limit?: number; offset?: number },
): Promise<AdminPaged<AdminClientChatFeedbackItem>> {
  const query = buildQuery({ rating: params?.rating, limit: params?.limit, offset: params?.offset });
  return apiGet<AdminPaged<AdminClientChatFeedbackItem>>(`/api/v1/admin/clients/${sellerAccountId}/feedback/chat${query}`);
}

export async function getAdminGlobalChatFeedback(params?: {
  seller_account_id?: number;
  rating?: string;
  limit?: number;
  offset?: number;
}): Promise<AdminPaged<AdminClientChatFeedbackItem>> {
  const query = buildQuery({
    seller_account_id: params?.seller_account_id,
    rating: params?.rating,
    limit: params?.limit,
    offset: params?.offset,
  });
  return apiGet<AdminPaged<AdminClientChatFeedbackItem>>(`/api/v1/admin/feedback/chat${query}`);
}

export type AdminRecommendationFeedbackResponse = {
  items: Array<{
    id: number;
    seller_account_id: number;
    recommendation_id: number;
    rating: string;
    comment?: string | null;
    created_at: string;
    recommendation: {
      id: number;
      recommendation_type: string;
      title: string;
      priority_level: string;
      confidence_level: string;
      status: string;
      entity_type: string;
      entity_id?: string | null;
      entity_sku?: number | null;
      entity_offer_id?: string | null;
      created_at: string;
    };
  }>;
  proxy_status_feedback: {
    accepted_count: number;
    dismissed_count: number;
    resolved_count: number;
  };
  limitations: string[];
  limit: number;
  offset: number;
};

export async function getAdminRecommendationFeedback(
  sellerAccountId: number,
  params?: { rating?: string; status?: string; limit?: number; offset?: number },
): Promise<AdminRecommendationFeedbackResponse> {
  const query = buildQuery({
    rating: params?.rating,
    status: params?.status,
    limit: params?.limit,
    offset: params?.offset,
  });
  return apiGet<AdminRecommendationFeedbackResponse>(
    `/api/v1/admin/clients/${sellerAccountId}/feedback/recommendations${query}`,
  );
}

export type AdminBillingState = {
  seller_account_id: number;
  plan_code: string;
  status: string;
  trial_ends_at?: string | null;
  current_period_start?: string | null;
  current_period_end?: string | null;
  ai_tokens_limit_month?: number | null;
  ai_tokens_used_month: number;
  estimated_ai_cost_month: number;
  notes?: string | null;
  created_at: string;
  updated_at: string;
};

export async function getAdminClientBilling(sellerAccountId: number): Promise<AdminBillingState> {
  return apiGet<AdminBillingState>(`/api/v1/admin/clients/${sellerAccountId}/billing`);
}

export async function updateAdminClientBilling(
  sellerAccountId: number,
  payload: {
    plan_code: string;
    status: string;
    trial_ends_at?: string | null;
    current_period_start?: string | null;
    current_period_end?: string | null;
    ai_tokens_limit_month?: number | null;
    ai_tokens_used_month: number;
    estimated_ai_cost_month: number;
    notes?: string | null;
  },
): Promise<{ billing: AdminBillingState; action?: Record<string, unknown> | null }> {
  return apiPut<{ billing: AdminBillingState; action?: Record<string, unknown> | null }>(
    `/api/v1/admin/clients/${sellerAccountId}/billing`,
    payload,
  );
}

export async function getAdminBilling(params?: {
  status?: string;
  limit?: number;
  offset?: number;
}): Promise<AdminPaged<AdminBillingState>> {
  const query = buildQuery({ status: params?.status, limit: params?.limit, offset: params?.offset });
  return apiGet<AdminPaged<AdminBillingState>>(`/api/v1/admin/billing${query}`);
}

export type AdminActionsGenerationRun = {
  run_id: number;
  seller_account_id: number;
  metric_date?: string;
  status: string;
  final_status?: string;
  context_hash?: string;
  accepted_count: number;
  repaired_count: number;
  dropped_count: number;
  fallback_count: number;
  created_at?: string;
  updated_at?: string;
  ai_focus_json?: unknown;
  top_action_keys_json?: unknown;
  data_quality_notes_json?: unknown;
  ai_pipeline_run_id?: number;
  data_pack_hash?: string;
  global_context_hash?: string;
};

export type AdminPipelineDiagnostics = {
  pipeline_run_id: number;
  seller_account_id: number;
  metric_date?: string;
  data_pack_hash?: string;
  global_context_hash?: string;
  dashboard_run_id?: string;
  actions_run_id?: number;
  dashboard_status?: string;
  actions_status?: string;
  ai_proxy: {
    ai_proxy_configured: boolean;
    ai_proxy_host?: string;
    effective_proxy_source: string;
  };
  dashboard_top_actions: {
    source?: string;
    action_ids?: number[];
    actions_run_id?: number;
  };
  artifacts: string[];
  warnings: string[];
};

export type AdminActionsGenerationCategoryRun = {
  domain: string;
  status: string;
  model?: string;
  accepted_count: number;
  repaired_count: number;
  dropped_count: number;
  validation_json?: unknown;
  final_validation_json?: unknown;
  raw_response_json?: unknown;
  repair_response_json?: unknown;
  created_at?: string;
  updated_at?: string;
};

export async function getAdminActionGenerationRuns(params?: {
  seller_id?: number;
  status?: string;
  final_status?: string;
  metric_date?: string;
  limit?: number;
  offset?: number;
}): Promise<AdminPaged<AdminActionsGenerationRun>> {
  const query = buildQuery({
    seller_id: params?.seller_id,
    status: params?.status,
    final_status: params?.final_status,
    metric_date: params?.metric_date,
    limit: params?.limit,
    offset: params?.offset,
  });
  return apiGet<AdminPaged<AdminActionsGenerationRun>>(`/api/v1/admin/actions/generation-runs${query}`);
}

export async function getAdminActionGenerationRunDetail(
  runId: number,
): Promise<{ run: AdminActionsGenerationRun; category_runs: AdminActionsGenerationCategoryRun[] }> {
  return apiGet<{ run: AdminActionsGenerationRun; category_runs: AdminActionsGenerationCategoryRun[] }>(
    `/api/v1/admin/actions/generation-runs/${runId}`,
  );
}

export async function getAdminPipelineDiagnostics(pipelineRunId: number): Promise<AdminPipelineDiagnostics> {
  return apiGet<AdminPipelineDiagnostics>(`/api/v1/admin/ai-pipeline/${pipelineRunId}/diagnostics`);
}

export async function rerunAdminSync(
  sellerAccountId: number,
  payload: { sync_type?: string },
): Promise<AdminActionExecutionResult> {
  return performAdminActionRequest(`/api/v1/admin/clients/${sellerAccountId}/actions/rerun-sync`, payload);
}

export async function resetAdminCursor(
  sellerAccountId: number,
  payload: { domain: string; cursor_type: string; cursor_value?: string | null },
): Promise<AdminActionExecutionResult> {
  return performAdminActionRequest(`/api/v1/admin/clients/${sellerAccountId}/actions/reset-cursor`, payload);
}

export async function rerunAdminMetrics(
  sellerAccountId: number,
  payload: { date_from: string; date_to: string },
): Promise<AdminActionExecutionResult> {
  return performAdminActionRequest(`/api/v1/admin/clients/${sellerAccountId}/actions/rerun-metrics`, payload);
}

export async function rerunAdminAlerts(
  sellerAccountId: number,
  payload: { as_of_date: string },
): Promise<AdminActionExecutionResult> {
  return performAdminActionRequest(`/api/v1/admin/clients/${sellerAccountId}/actions/rerun-alerts`, payload);
}

export async function rerunAdminRecommendations(
  sellerAccountId: number,
  payload: { as_of_date: string },
): Promise<AdminActionExecutionResult> {
  return performAdminActionRequest(`/api/v1/admin/clients/${sellerAccountId}/actions/rerun-recommendations`, payload);
}

export type AdminActionLog = {
  id: number;
  admin_user_id?: number | null;
  admin_email: string;
  seller_account_id: number;
  action_type: string;
  target_type?: string | null;
  target_id?: number | null;
  request_payload: Record<string, unknown>;
  result_payload: Record<string, unknown>;
  status: string;
  error_message?: string | null;
  created_at: string;
  finished_at?: string | null;
};

export type AdminActionExecutionResult = {
  ok: boolean;
  statusCode: number;
  action: AdminActionLog | null;
  error: string | null;
};

async function performAdminActionRequest(
  path: string,
  payload: unknown,
): Promise<AdminActionExecutionResult> {
  const res = await fetch(`${PUBLIC_API_BASE_URL}${path}`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  const contentType = res.headers.get("content-type") || "";
  const isJson = contentType.includes("application/json");
  const body = isJson ? ((await res.json()) as unknown) : null;

  if (isAdminActionLog(body)) {
    return {
      ok: res.ok,
      statusCode: res.status,
      action: body,
      error: res.ok ? null : body.error_message || "Action failed",
    };
  }

  const payloadError =
    body && typeof body === "object" && "error" in body && typeof (body as { error?: unknown }).error === "string"
      ? ((body as { error: string }).error ?? null)
      : null;
  return {
    ok: res.ok,
    statusCode: res.status,
    action: null,
    error: payloadError || (res.ok ? null : `Request failed with status ${res.status}`),
  };
}

function isAdminActionLog(v: unknown): v is AdminActionLog {
  if (!v || typeof v !== "object") return false;
  const r = v as Record<string, unknown>;
  return (
    typeof r.id === "number" &&
    typeof r.action_type === "string" &&
    typeof r.status === "string" &&
    typeof r.seller_account_id === "number"
  );
}
