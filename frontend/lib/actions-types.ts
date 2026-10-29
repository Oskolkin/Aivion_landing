export type ActionStatus = "open" | "checked" | "snoozed" | "dismissed";
export type ActionPriority = "critical" | "important" | "watch";
export type ActionCategory =
  | "stock"
  | "slow_turnover"
  | "sku"
  | "sales"
  | "advertising"
  | "funnel"
  | "category"
  | "data_quality";
export type ActionType =
  | "stock_replenishment"
  | "stock_redistribution"
  | "slow_turnover_review"
  | "no_sales_with_stock"
  | "sku_orders_drop"
  | "key_sku_risk"
  | "key_sku_protection"
  | "ad_high_drr"
  | "ad_high_drr_review"
  | "ad_spend_no_orders"
  | "ad_budget_reallocation"
  | "funnel_weak_transition"
  | "card_conversion_review"
  | "category_underperformance"
  | "sales_drop_review"
  | "data_quality_issue";

export type ActionsResponseStatus = "available" | "empty" | "partial" | "error";

export type ActionEvidence = {
  kind: string;
  label: string;
  value: string;
  unit?: string;
  direction?: string;
  description?: string;
};

export type ActionCTA = {
  type: string;
  label: string;
  href?: string;
  export_type?: string;
  prompt?: string;
  params?: Record<string, string>;
};

export type ActionSKURef = {
  sku: string;
  offer_id?: string;
  name?: string;
  image_url?: string;
};

export type ActionCampaignRef = {
  campaign_id: string;
  name?: string;
};

export type ActionFunnelRef = {
  transition_key: string;
  from_stage: string;
  to_stage: string;
};

export type ActionAffectedEntities = {
  skus?: ActionSKURef[];
  campaigns?: ActionCampaignRef[];
  funnel?: ActionFunnelRef | null;
};

export type ActionsAIFocus = {
  status: string;
  source: string;
  headline: string;
  summary: string;
  top_categories?: string[];
  limitations?: string[];
  start_with?: string;
  priority_order?: string[];
};

export type ActionCounters = {
  total_open: number;
  critical: number;
  important: number;
  watch: number;
  stock: number;
  sku: number;
  advertising: number;
  funnel: number;
  sales: number;
  checked: number;
  slow_turnover: number;
  category_count: number;
  dismissed: number;
};

export type OperationalSecondaryAction = {
  type: string;
  label: string;
};

export type OperationalFact = {
  fact_id: string;
  label: string;
  value: string;
  unit?: string;
  formatted_value?: string;
};

export type OperationalStep = {
  step_id: string;
  action_kind?: string;
  title: string;
  instruction: string;
  why_this_step: string;
  expected_result: string;
  confidence?: string;
  decision_mode?: string;
  evidence_fact_ids?: string[];
  target_entities?: Array<{ entity_type: string; id: string; name?: string }>;
};

export type OperationalObjectAction = {
  entity_type: string;
  entity_id: string;
  name?: string;
  metrics?: Array<{ label: string; value: string; metric_code?: string; display_label?: string; formatted_value?: string }>;
  recommended_action: string;
  reason?: string;
  confidence?: string;
};

export type OperationalDetail = {
  status: string;
  format_version: string;
  why: {
    headline: string;
    text: string;
    facts?: OperationalFact[];
  };
  action_plan: {
    headline: string;
    steps: OperationalStep[];
  };
  object_actions?: OperationalObjectAction[];
  secondary_actions?: OperationalSecondaryAction[];
  hidden_limitations?: string[];
};

export type ActionScopeType = "account" | "entity" | "mixed";

export type ActionObjectActionsCoverage = {
  mode: "full" | "partial" | "none" | string;
  reason?: string;
};

export type ActionCompletionCondition = {
  metric_code: string;
  operator: string;
  target_value?: number;
  target_value_max?: number;
  fact_id?: string;
  scope: string;
  period_days?: number;
  display_text?: string;
};

export type ActionItem = {
  id: number;
  action_key: string;
  action_view_id?: string;
  type: ActionType;
  category: ActionCategory;
  priority: ActionPriority;
  /** required | recommended from unified analysis; optional on legacy rows */
  execution_tier?: "required" | "recommended" | string;
  status: ActionStatus;
  title: string;
  summary?: string;
  reason: string;
  next_step: string;
  expected_effect?: string;
  evidence: ActionEvidence[];
  limitations?: string[];
  ctas: ActionCTA[];
  affected_entities?: ActionAffectedEntities;
  source_candidate_ids?: string[];
  evidence_fact_ids?: string[];
  entity_group_ref?: string;
  operational_detail?: OperationalDetail;
  urgency_marker?: string;
  affected_objects_summary?: string;
  affected_objects_total?: number;
  affected_objects_displayed?: number;
  affected_objects_truncated?: boolean;
  operational_goal?: string;
  scope_type?: ActionScopeType;
  object_actions_coverage?: ActionObjectActionsCoverage;
  completion_criteria?: string;
  completion_conditions?: ActionCompletionCondition[];
  visible_limitations?: string[];
  metric_date?: string;
  run_id?: number;
  source?: {
    publication_id?: number;
    pipeline_run_id?: number;
    actions_run_id?: number;
    data_pack_hash?: string;
    global_context_hash?: string;
    pack_hash?: string;
    metric_date?: string;
  };
  created_at?: string;
  updated_at?: string;
  checked_at?: string;
};

export type ActionsResponse = {
  metric_date?: string;
  status: ActionsResponseStatus;
  ai_focus?: ActionsAIFocus;
  counters: ActionCounters;
  actions: ActionItem[];
  data_quality_notes?: string[];
  analysis_source?: string;
  publication_id?: number;
  pipeline_run_id?: number;
  fallback_used?: boolean;
  fallback_reason?: string;
};

export type ListActionsParams = {
  status?: "open" | "checked" | "snoozed" | "dismissed" | "all";
  priority?: ActionPriority;
  category?: ActionCategory;
  metric_date?: string;
  limit?: number;
  offset?: number;
};

export type ActionsFilterKey =
  | "open"
  | "stock"
  | "slow_turnover"
  | "sku_sales"
  | "advertising"
  | "funnel"
  | "category"
  | "checked"
  | "dismissed";
