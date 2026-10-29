export type WidgetStatus =
  | "available"
  | "partial"
  | "not_enough_data"
  | "source_not_available"
  | "source_access_denied"
  | "premium_required"
  | "source_pending"
  | "error";

export type GlobalStatus = "ready" | "partial" | "not_enough_data";

export type WidgetBase = {
  status: WidgetStatus;
  message?: string;
};

/** API may return null for array fields when AI summary JSON is partial or empty. */
export type NullableArray<T> = T[] | null;

export type SparklinePoint = {
  date: string;
  value?: number | null;
  revenue?: number | null;
  has_data?: boolean;
};

export type ActionItem = {
  priority: string;
  urgency_group: string;
  action: string;
  reason?: string;
  expected_effect?: string;
  related_entity_type?: string;
  related_entity_id?: string | null;
  source: string;
  linked_alert_id?: number | null;
  linked_recommendation_id?: number | null;
};

export type WhatChangedItem = {
  label: string;
  text: string;
  highlights?: NullableArray<string>;
  domain?: string;
  severity?: string;
  current_value_text?: string;
  previous_value_text?: string;
  delta_text?: string;
  impact_text?: string;
  entity_count?: number;
  entity_names?: NullableArray<string>;
  linked_action_id?: string | null;
  action_link_label?: string;
  deep_link?: string;
  change_id?: string;
};

export type WhatChangedGroup = {
  key: "worsened" | "new_risks" | "unchanged_risks" | "improved" | string;
  label: string;
  tone: "negative" | "warning" | "neutral" | "positive" | string;
  summary?: string;
  items?: NullableArray<WhatChangedItem>;
  empty_text?: string;
};

export type TopActionItem = {
  id: string;
  action_id?: number;
  action_key?: string;
  actions_run_id?: number;
  pipeline_run_id?: number;
  type?: string;
  summary?: string;
  completion_criteria?: string;
  visible_limitations?: NullableArray<string>;
  driver_id?: string;
  source: "recommendation" | "ai_summary" | "alert" | "risk_driver" | "actions_final" | string;
  priority: "critical" | "high" | "medium" | "low";
  priority_label: string;
  category: "advertising" | "stock" | "slow_turnover" | "sales" | "funnel" | "cancellations" | "products" | "account" | "other" | string;
  category_label: string;
  title: string;
  reason: string;
  next_step: string;
  expected_effect?: string;
  linked_alert_ids?: NullableArray<number>;
  linked_recommendation_ids?: NullableArray<number>;
  supporting_metrics?: NullableArray<{
    label: string;
    value: string;
  }>;
};

export type RiskDriver = {
  id: string;
  category: "sales" | "cancellations" | "funnel" | "stock" | "surplus" | "advertising" | "sku" | "account";
  category_label: string;
  severity: "critical" | "high" | "medium" | "low";
  impact_score: number;
  title: string;
  reason: string;
  evidence?: Record<string, unknown> | null;
  linked_alert_ids?: NullableArray<number>;
  linked_recommendation_ids?: NullableArray<number>;
};

export type HealthScoreBreakdownItem = {
  driver_id: string;
  category: string;
  severity: "critical" | "high" | "medium" | "low";
  impact_score: number;
  weight: number;
  severity_base: number;
  impact_part: number;
  raw_driver_penalty: number;
  capped_driver_penalty: number;
};

export type HealthScoreBreakdown = {
  drivers: NullableArray<HealthScoreBreakdownItem>;
  raw_penalty: number;
  saturated_penalty: number;
  final_score: number;
};

export type CategorySummaryCard = {
  category_id?: string;
  category_name: string;
  category_score?: number | null;
  score_max?: number;
  health_score?: number | null;
  severity?: string;
  headline?: string;
  key_risk?: string;
  recommended_action?: string;
};

export type FunnelStage = {
  key: string;
  label: string;
  value?: number | null;
  conversion_from_previous?: number | null;
  conversion_from_top?: number | null;
  previous_day_value?: number | null;
  change_vs_previous_day_pct?: number | null;
  /** Same transition conversion on the previous completed day (D-2). */
  previous_conversion_from_previous?: number | null;
  /** D-1 conversion minus D-2 conversion, in percentage points. */
  conversion_delta_pp?: number | null;
  /** Factual volume window marker; Funnel detail uses completed_day. */
  period_type?: string | null;
};

export type FunnelDailyPoint = {
  date: string;
  impressions: number | null;
  product_views: number | null;
  add_to_cart: number | null;
  ordered_units: number | null;
  delivered_units: number | null;
};

export type FunnelWeeklyStage = {
  key: string;
  label: string;
  value?: number | null;
  previous_period_value?: number | null;
  change_vs_previous_period_pct?: number | null;
  conversion_from_previous?: number | null;
  previous_conversion_from_previous?: number | null;
  conversion_delta_pp?: number | null;
};

export type FunnelAIAnalysis = WidgetBase & {
  short_summary?: string;
  summary?: string;
  main_conclusion?: string;
  action_items?: NullableArray<string>;
  trend_vs_yesterday?: string;
  trend_vs_7d?: string;
  related_signals?: NullableArray<string>;
  recommended_focus?: NullableArray<string>;
  full_analysis?: string;
  details?: string;
  weakest_stage?: {
    status?: "available" | "stable" | "unavailable" | string;
    key?: string;
    label?: string;
    metric?: string;
    value?: string;
    explanation?: string;
    reason?: string;
    delta_previous?: string;
    delta_7d?: string;
    tone?: "negative" | "warning" | "neutral" | "positive" | string;
  };
  comparisons?: {
    previous_period?: NullableArray<{
      key?: string;
      label?: string;
      value?: string;
      delta?: string;
      tone?: "negative" | "warning" | "neutral" | "positive" | string;
    }>;
    seven_day_average?: NullableArray<{
      key?: string;
      label?: string;
      value?: string;
      delta?: string;
      tone?: "negative" | "warning" | "neutral" | "positive" | string;
    }>;
    stage_metrics?: NullableArray<{
      key?: string;
      label?: string;
      value?: string;
      delta?: string;
      tone?: "negative" | "warning" | "neutral" | "positive" | string;
    }>;
    conversion_metrics?: NullableArray<{
      key?: string;
      label?: string;
      value?: string;
      delta?: string;
      tone?: "negative" | "warning" | "neutral" | "positive" | string;
    }>;
    business_metrics?: NullableArray<{
      key?: string;
      label?: string;
      value?: string;
      delta?: string;
      tone?: "negative" | "warning" | "neutral" | "positive" | string;
    }>;
    context_signals?: NullableArray<{
      key?: string;
      label?: string;
      value?: string;
      delta?: string;
      tone?: "negative" | "warning" | "neutral" | "positive" | string;
    }>;
  };
  comparisons_commentary?: {
    previous_period?: string;
    seven_day_average?: string;
    previous_summary?: string;
  };
  drop_factors?: NullableArray<{
    key?: string;
    label?: string;
    group?: "funnel_signal" | "business_signal" | "possible_amplifier" | "context_hypothesis" | string;
    metric?: string;
    explanation?: string;
    evidence_key?: string;
    evidence_level?: "confirmed" | "likely" | "hypothesis" | "limited" | "unknown" | string;
    delta?: string;
    limitation?: string;
    description?: string;
    tone?: "negative" | "warning" | "neutral" | "positive" | string;
    text?: string;
  }>;
  details_sections?: NullableArray<{
    title?: string;
    items?: NullableArray<{
      text?: string;
      metrics?: NullableArray<string>;
    }>;
  }>;
  influence_factors?: NullableArray<{
    key?: string;
    label?: string;
    tone?: "negative" | "warning" | "neutral" | "positive" | string;
    text?: string;
  }>;
  check_directions?: NullableArray<{
    key?: string;
    label?: string;
    tone?: "negative" | "warning" | "neutral" | "positive" | string;
    text?: string;
    priority?: string;
    reason?: string;
    data_needed?: string;
  }>;
};

export type StockCoverageSegment = {
  key: string;
  label: string;
  sku_count: number;
  severity: string;
};

export type StockCoverageNarrative = {
  status: string;
  headline?: string;
  summary?: string;
  next_step?: string;
  tone?: string;
  source?: string;
  no_sales_insight?: string;
  risk_comment?: string;
  actions?: NullableArray<{
    title: string;
    text?: string;
    priority?: string;
    target?: {
      type: string;
      tab?: string;
    } | null;
  }>;
  checklist?: NullableArray<{
    title: string;
    text?: string;
    priority?: string;
    target?: {
      type: string;
      tab?: string;
    } | null;
  }>;
  revenue_at_risk: {
    status: string;
    label?: string;
    value?: string;
    method_note?: string;
    share_of_daily_revenue_pct?: number | null;
  };
  segments?: NullableArray<{
    key: string;
    label: string;
    count: number;
    severity: string;
    comment?: string;
  }>;
  risk_examples?: NullableArray<{
    product_name: string;
    stock_label: string;
    coverage_label: string;
    risk_label?: string;
    reason?: string;
  }>;
  no_sales_with_stock: {
    status: string;
    count?: number;
    shown_count?: number;
    stock_units?: number;
    summary?: string;
    comment?: string;
    target?: string;
  };
  advertising_link?: {
    status: string;
    comment?: string;
  };
  sku_focus_link: {
    label: string;
    target: "sku-focus-stock" | string;
  };
  no_sales_focus_link?: {
    label: string;
    target: "sku-focus-no-sales" | string;
  } | null;
};

export type AdEfficiencyPoint = {
  campaign_external_id: number;
  campaign_name: string;
  spend: number;
  drr?: number | null;
  revenue: number;
  orders: number;
  efficiency_signal: string;
};

export type AdEfficiencyPeriod = {
  metric_date: string;
  date_from: string;
  date_to: string;
  current_completed_date?: string;
  previous_completed_date?: string;
  current_7d_date_from?: string;
  current_7d_date_to?: string;
  previous_7d_date_from?: string;
  previous_7d_date_to?: string;
  sample_days: number;
  previous_sample_days?: number;
};

export type AdEfficiencySummary = {
  current_drr_pct?: number | null;
  previous_drr_pct?: number | null;
  average_drr_7d_pct?: number | null;
  delta_vs_previous_pp?: number | null;
  delta_vs_average_7d_pp?: number | null;
  previous_drr_7d_pct?: number | null;
  delta_vs_previous_7d_pp?: number | null;
  total_spend_today: number;
  /** Attributed ad revenue today (denominator for ads share). */
  total_revenue_today: number;
  total_store_revenue_today?: number;
  total_orders_today: number;
  total_spend_7d?: number;
  total_attributed_revenue_7d?: number;
  total_store_revenue_7d?: number;
  campaigns_high_drr_count: number;
  campaigns_no_orders_count: number;
  high_drr_spend?: number;
  spend_without_orders: number;
  spend_without_orders_date_from?: string;
  spend_without_orders_date_to?: string;
  high_drr_threshold_pct: number;
};

export type AdEfficiencyTrendPoint = {
  date: string;
  label?: string;
  spend: number;
  revenue: number;
  orders: number;
  drr_pct?: number | null;
  is_today?: boolean;
  is_yesterday?: boolean;
  data_status: string;
};

export type AdCampaignDisplayRow = {
  campaign_external_id: number;
  campaign_name: string;
  drr_pct?: number | null;
  spend: number;
  orders: number;
  revenue: number;
  roas?: number | null;
  cpo?: number | null;
  ctr?: number | null;
  cpc?: number | null;
  click_to_order_rate?: number | null;
  reason_code: string;
};

export type AdEfficiencyNarrative = {
  status: string;
  /** Short thesis headline — never derived by splitting summary on '.'. */
  headline?: string;
  summary?: string;
  /** Actionable recommendation grounded in problem campaigns — AI-generated only. */
  recommendation?: string;
  checklist?: NullableArray<AdEfficiencyCheckItem>;
  limitations?: NullableArray<string>;
};

export type AdEfficiencyCheckItem = {
  title: string;
  text?: string;
  priority?: string;
  target?: string;
  supporting_metric_refs?: NullableArray<string>;
};

export type AdEfficiencyCTA = {
  label: string;
  href: string;
};

export type RiskGroupCount = {
  group: string;
  label: string;
  count: number;
  severity: string;
};

export type KPICard = {
  key: string;
  label: string;
  value: string;
  delta?: {
    abs: number;
    pct?: number | null;
    label: string;
    unit?: string;
    display?: string;
  } | null;
  sparkline_7d?: NullableArray<SparklinePoint>;
  trend?: {
    status?: string;
    label?: string;
  } | null;
  hint?: string;
};

export type SellerPulseNarrativeStatus =
  | "ready"
  | "pending"
  | "stale"
  | "fallback"
  | "not_enough_data";

export type SellerPulseNarrative = {
  status: SellerPulseNarrativeStatus;
  tone?: "stable" | "attention" | "critical" | "neutral" | string;
  headline: string;
  summary?: string;
  /** Bullet points inside AI-резюме. */
  key_points?: NullableArray<SellerPulseImportantNote>;
  /** Seller-facing notes for «Что важно знать» — separate from key_points. */
  important_notes?: NullableArray<SellerPulseImportantNote>;
  quick_actions?: NullableArray<SellerPulseQuickAction>;
  chips?: NullableArray<SellerPulseChip>;
  data_quality_note?: string;
};

export type SellerPulseImportantNote = {
  title?: string;
  text: string;
  tone?: "positive" | "warning" | "critical" | "neutral" | "negative" | "risk" | string;
  metric_refs?: NullableArray<string>;
  fact_ids?: NullableArray<string>;
  entity_ids?: NullableArray<string>;
};

export type SellerPulseQuickAction = {
  label: string;
  description?: string;
  priority?: "high" | "medium" | "low" | string;
  kind?: string;
  href?: string;
  action_id?: string | null;
  analysis_action_id?: string | null;
};

export type SellerPulseChip = {
  label: string;
  tone: "positive" | "warning" | "critical" | "neutral" | string;
};

export type StockTrendPoint = {
  date: string;
  stock: number;
  coverage_days?: number | null;
  has_data?: boolean;
};

export type KeySkuRevenueTrendPoint = {
  date: string;
  revenue: number;
  orders: number;
  has_data?: boolean;
};

export type KeySkuCard = {
  ozon_product_id: number;
  product_name: string;
  sku?: number | null;
  offer_id?: string | null;
  image_url?: string | null;
  product_url?: string | null;
  revenue_7d: number;
  orders_7d: number;
  revenue_share_pct: number;
  cumulative_revenue_share_pct: number;
  coverage_days?: number | null;
  current_stock?: number | null;
  revenue_trend_7d: NullableArray<KeySkuRevenueTrendPoint>;
};

export type SKUFocusCard = {
  ozon_product_id: number;
  sku?: number | null;
  offer_id?: string | null;
  product_name: string;
  image_url?: string | null;
  problem: string;
  key_metric: string;
  trend_status?: "available" | "not_enough_data" | "source_not_available" | string | null;
  trend_message?: string | null;
  sparkline_7d?: NullableArray<SparklinePoint>;
  stock_trend_7d?: NullableArray<StockTrendPoint>;
  product_url?: string | null;
  current_stock?: number | null;
  previous_stock?: number | null;
  stock_delta_day_to_day?: number | null;
  coverage_days?: number | null;
  /** Canonical stockpolicy segment from backend (e.g. critical_0_3). */
  coverage_bucket?: string | null;
  avg_daily_revenue_7d?: number | null;
  revenue_at_risk_7d?: number | null;
  reason?: string;
  check_hint?: string;
  next_step?: string;
  recommended_action?: string;
  note?: string;
  explanation?: string;
  summary?: string;
  driver?: string;
  previous_orders?: number | null;
  current_orders?: number | null;
  orders_previous?: number | null;
  orders_current?: number | null;
  orders_before?: number | null;
  orders_after?: number | null;
  orders_delta_day_to_day?: number | null;
  revenue_delta_day_to_day?: number | null;
  revenue_delta?: number | null;
  revenue_change?: number | null;
  revenue_loss?: number | null;
  revenue_impact?: number | null;
  lost_revenue?: number | null;
  revenue?: number | string | null;
  no_sales_days?: number | null;
  last_completed_metric_date?: string | null;
  unit_price?: number | null;
  frozen_stock_value?: number | null;
  previous_day_orders?: number | null;
};

export type DashboardPipelineInfo = {
  pipeline_run_id?: number;
  data_pack_hash?: string;
  global_context_hash?: string;
  dashboard_run_id?: string;
  actions_run_id?: number;
  top_actions_source?: string;
};

export type DashboardHomeResponse = {
  metric_date: string;
  status: GlobalStatus;
  data_quality_notes: NullableArray<string>;
  pipeline?: DashboardPipelineInfo | null;
  analysis_source?: string;
  publication_id?: number;
  fallback_used?: boolean;
  fallback_reason?: string;
  dashboard_content_status?: string;
  requested_date?: string;
  effective_metric_date?: string;
  latest_complete_data_date?: string;
  data_freshness_lag_days?: number;
  data_is_stale?: boolean;
  historical_analysis?: boolean;
  analysis_age_days?: number;
  source_freshness_lag_days?: number;
  top_actions?: NullableArray<TopActionItem>;
	period_context?: {
    value_period?: string;
    comparison_period?: string;
    trend_period?: string;
    current_range?: string;
    previous_range?: string;
    caption?: string;
    caption_primary?: string;
    caption_secondary?: string;
  } | null;
  ai_diagnostics: {
    account_summary: WidgetBase & {
      health_score?: number | null;
      health_score_delta?: number | null;
      severity?: "critical" | "high" | "medium" | "low" | "attention" | string;
      health_status_code?: string;
      health_status_label?: string;
      health_status_explanation?: string;
      headline?: string;
      executive_summary?: string;
      improvement_summary?: string;
      improvement_levers?: NullableArray<{
        title: string;
        explanation: string;
        factor_id?: string;
        linked_action_id?: string | null;
        expected_direction?: string;
      }>;
      risk_drivers?: NullableArray<RiskDriver>;
      health_score_breakdown?: HealthScoreBreakdown | null;
      metric_date?: string;
      freshness?: string;
      generated_at?: string;
      narrative_source?: "ai" | "fallback" | string;
      narrative_run_id?: string;
      prompt_version?: string;
      validation_status?: string;
    };
    what_changed: WidgetBase & {
      summary?: string;
      main_conclusion?: string;
      groups?: NullableArray<WhatChangedGroup>;
    };
    top_actions_source?: string;
    top_actions: NullableArray<TopActionItem>;
    category_summaries: NullableArray<CategorySummaryCard>;
    sales_funnel: WidgetBase & {
      current_completed_date: string;
      previous_completed_date: string;
      stages: NullableArray<FunnelStage>;
      daily_series: NullableArray<FunnelDailyPoint>;
      overview_7d?: (WidgetBase & { stages?: NullableArray<FunnelWeeklyStage> }) | null;
    };
    funnel_analysis: WidgetBase & {
      primary_drop_stage?: string;
      primary_drop_message?: string;
      ai_analysis?: FunnelAIAnalysis;
    };
    stock_coverage: WidgetBase & {
      segments?: NullableArray<StockCoverageSegment>;
      total_skus?: number;
      stock_coverage_narrative?: StockCoverageNarrative | null;
    };
    ad_efficiency: WidgetBase & {
      points?: NullableArray<AdEfficiencyPoint>;
      period?: AdEfficiencyPeriod | null;
      summary?: AdEfficiencySummary | null;
      trend_7d?: NullableArray<AdEfficiencyTrendPoint>;
      high_drr_campaigns?: NullableArray<AdCampaignDisplayRow>;
      no_order_campaigns?: NullableArray<AdCampaignDisplayRow>;
      ad_efficiency_narrative?: AdEfficiencyNarrative | null;
      cta?: AdEfficiencyCTA | null;
    };
    risk_distribution: WidgetBase & {
      groups?: NullableArray<RiskGroupCount>;
      total?: number;
    };
  };
  seller_pulse: {
    kpis: NullableArray<KPICard>;
    trend_7d: NullableArray<{ date: string; revenue?: number | null; orders?: number | null }>;
    narrative?: SellerPulseNarrative | null;
  };
  sku_focus: {
    order_drop: NullableArray<SKUFocusCard>;
    stock_risk: NullableArray<SKUFocusCard>;
    stock_risk_total_count?: number;
    no_sales_with_stock: NullableArray<SKUFocusCard>;
    no_sales_with_stock_total_count?: number;
    surplus: NullableArray<SKUFocusCard>;
    key_skus: NullableArray<KeySkuCard>;
    key_skus_total_count?: number;
    store_total_revenue_7d?: number;
    key_skus_total_revenue_7d?: number;
  };
  recommended_actions: NullableArray<ActionItem>;
};

export type SKUFocusGroupKey =
  | "order_drop"
  | "stock_risk"
  | "no_sales_with_stock"
  | "surplus"
  | "key_skus";
