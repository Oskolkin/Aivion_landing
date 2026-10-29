export type ProductState =
  | "READY"
  | "PRE_ANALYSIS"
  | "PARTIAL"
  | "UNAVAILABLE"
  | "NOT_APPLICABLE"
  | "FAILED"
  | "READY_EMPTY";

export type PublicationMeta = {
  id: string;
  analysis_date: string;
  analysis_cutoff_at: string;
  published_at: string;
};

export type TrendPoint = {
  date: string;
  amount?: string | null;
  state: string;
};

export type KPI = {
  key: string;
  status: string;
  amount?: string | null;
  unit?: string;
  current?: string | null;
  previous?: string | null;
  delta?: string | null;
  delta_unit?: string;
  reason_code?: string;
  trend?: TrendPoint[];
};

export type FindingView = {
  ref?: string;
  title: string;
  statement: string;
  primary_domain: string;
  overview_bucket: string;
  entity_kind?: string;
  entity_id?: string;
};

export type TakeawayView = {
  statement: string;
  primary_domain: string;
};

export type OverviewProjection = {
  schema_version: string;
  seller_account_id: number;
  analysis_date: string;
  analysis_cutoff_at: string;
  kpis: KPI[];
  executive_headline?: string;
  executive_summary: string;
  takeaways?: TakeawayView[];
  findings?: FindingView[];
  sku_focus_preview?: Record<string, unknown>[];
  limitations?: string[];
};

export type FunnelStage = {
  key: string;
  status: string;
  amount?: string | null;
  previous_amount?: string | null;
  delta?: string | null;
  reason_code?: string;
};

export type FunnelTransition = {
  from_key: string;
  to_key: string;
  key: string;
  status: string;
  amount?: string | null;
};

export type FunnelProjection = {
  schema_version: string;
  state: ProductState | string;
  reason_code?: string;
  status: string;
  period: { current: { key: string }; previous: { key: string } };
  data_as_of?: string;
  stages: FunnelStage[];
  transitions: FunnelTransition[];
  insight?: { headline: string; statement: string; primary_domain: string } | null;
  limitations?: string[];
  related_action_local_refs?: string[];
};

export type CoverageBandCount = { key: string; count: number; state: string };

export type DomainProjection = {
  schema_version: string;
  state?: string;
  reason_code?: string;
  kpis?: KPI[];
  coverage_bands?: CoverageBandCount[];
  account_ad_spend_share?: KPI | null;
  yesterday_ad_spend_share?: KPI | null;
  findings?: FindingView[];
  finding_refs?: string[];
  recommendation_refs?: string[];
  action_local_refs?: string[];
  limitations?: string[];
};

export type FocusGroup = {
  action_view_id: string;
  display_order: number;
  display_title: string;
  display_summary: string;
  action_type: string;
  family: string;
  active_member_count: number;
};

export type PublicationDashboardResponse = {
  state: ProductState | string;
  publication: PublicationMeta | null;
  pre_analysis?: { reason_code: string; message_key: string };
  periods?: Record<string, unknown>;
  limitations?: string[];
  overview?: OverviewProjection | null;
  funnel?: FunnelProjection | null;
  stock?: DomainProjection | null;
  advertising?: DomainProjection | null;
  focus?: FocusGroup[];
  error?: { reason_code: string; message_key: string };
};

export type RowsResponse = {
  publication_id: string;
  items: Record<string, unknown>[];
  next_cursor?: string;
  has_more: boolean;
};
