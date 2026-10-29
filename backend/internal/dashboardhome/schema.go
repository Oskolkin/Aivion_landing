package dashboardhome

// WidgetStatus describes availability of a dashboard widget.
type WidgetStatus string

const (
	WidgetAvailable          WidgetStatus = "available"
	WidgetPartial            WidgetStatus = "partial"
	WidgetNotEnoughData      WidgetStatus = "not_enough_data"
	WidgetSourceNotAvailable WidgetStatus = "source_not_available"
	WidgetSourceAccessDenied WidgetStatus = "source_access_denied"
	WidgetPremiumRequired    WidgetStatus = "premium_required"
	WidgetSourcePending      WidgetStatus = "source_pending"
	WidgetError              WidgetStatus = "error"
)

// GlobalStatus describes overall dashboard home readiness.
type GlobalStatus string

const (
	StatusReady                GlobalStatus = "ready"
	StatusPartial              GlobalStatus = "partial"
	StatusNotEnoughData        GlobalStatus = "not_enough_data"
	StatusAIGenerationRequired GlobalStatus = "ai_generation_required"
)

// DashboardPipelineInfo links dashboard home to the unified AI pipeline run.
type DashboardPipelineInfo struct {
	PipelineRunID     int64  `json:"pipeline_run_id,omitempty"`
	DataPackHash      string `json:"data_pack_hash,omitempty"`
	GlobalContextHash string `json:"global_context_hash,omitempty"`
	DashboardRunID    string `json:"dashboard_run_id,omitempty"`
	ActionsRunID      int64  `json:"actions_run_id,omitempty"`
	TopActionsSource  string `json:"top_actions_source,omitempty"`
}

// Response is the contract for GET /api/v1/dashboard/home.
type Response struct {
	MetricDate       string                 `json:"metric_date"`
	Status           GlobalStatus           `json:"status"`
	DataQualityNotes []string               `json:"data_quality_notes"`
	Pipeline         *DashboardPipelineInfo `json:"pipeline,omitempty"`
	// AnalysisSource is "unified" | "legacy" | "none" for diagnostics (safe metadata).
	AnalysisSource string `json:"analysis_source,omitempty"`
	// PublicationID is set when AnalysisSource=unified.
	PublicationID int64 `json:"publication_id,omitempty"`
	// FallbackUsed is true when Unified mode fell back to legacy.
	FallbackUsed bool `json:"fallback_used,omitempty"`
	// FallbackReason explains why legacy/none was selected instead of unified.
	FallbackReason string `json:"fallback_reason,omitempty"`
	// DashboardContentStatus is a read-path diagnostic: ready | partial |
	// not_enough_data | ai_generation_required | error.
	DashboardContentStatus string `json:"dashboard_content_status,omitempty"`

	// Date freshness (Unified Analysis). MetricDate remains the primary display date
	// (= effective_metric_date). Extra fields explain lag vs requested/calendar date.
	RequestedDate          string `json:"requested_date,omitempty"`
	EffectiveMetricDate    string `json:"effective_metric_date,omitempty"`
	LatestCompleteDataDate string `json:"latest_complete_data_date,omitempty"`
	DataFreshnessLagDays   int    `json:"data_freshness_lag_days,omitempty"`
	DataIsStale            bool   `json:"data_is_stale,omitempty"`
	// HistoricalAnalysis/AnalysisAgeDays/SourceFreshnessLagDays distinguish a
	// deliberate fixed-date historical analysis from genuine source staleness.
	HistoricalAnalysis     bool `json:"historical_analysis,omitempty"`
	AnalysisAgeDays        int  `json:"analysis_age_days,omitempty"`
	SourceFreshnessLagDays int  `json:"source_freshness_lag_days,omitempty"`

	TopActions         []TopActionItem `json:"top_actions,omitempty"`
	AIDiagnostics      AIDiagnostics   `json:"ai_diagnostics"`
	SellerPulse        SellerPulse     `json:"seller_pulse"`
	SKUFocus           SKUFocus        `json:"sku_focus"`
	RecommendedActions []ActionItem    `json:"recommended_actions"`
	PeriodContext      *PeriodContext  `json:"period_context,omitempty"`
	// DashboardNarrativeDebug is populated when DASHBOARD_NARRATIVE_DEBUG=1.
	DashboardNarrativeDebug *DashboardNarrativeDebug `json:"dashboard_narrative_debug,omitempty"`

	blockApplySources     map[string]string `json:"-"`
	narrativeApplyRepairs []string          `json:"-"`
}

type WidgetBase struct {
	Status  WidgetStatus `json:"status"`
	Message string       `json:"message,omitempty"`
}

type AIDiagnostics struct {
	AccountSummary    AccountSummaryWidget   `json:"account_summary"`
	WhatChanged       WhatChangedWidget      `json:"what_changed"`
	TopActionsSource  string                 `json:"top_actions_source,omitempty"`
	TopActions        []TopActionItem        `json:"top_actions"`
	CategorySummaries []CategorySummaryCard  `json:"category_summaries"`
	SalesFunnel       SalesFunnelWidget      `json:"sales_funnel"`
	FunnelAnalysis    FunnelAnalysisWidget   `json:"funnel_analysis"`
	StockCoverage     StockCoverageWidget    `json:"stock_coverage"`
	AdEfficiency      AdEfficiencyWidget     `json:"ad_efficiency"`
	RiskDistribution  RiskDistributionWidget `json:"risk_distribution"`
	// AISummariesDebug is populated when DASHBOARD_NARRATIVE_DEBUG=1 or DASHBOARD_AI_SUMMARIES_DEBUG=1.
	AISummariesDebug *AISummariesDebug `json:"ai_summaries_debug,omitempty"`
}

// AISummariesDebug mirrors aisummaries.SummaryDebug for dashboard home JSON.
type AISummariesDebug struct {
	RunID              string             `json:"run_id,omitempty"`
	RunStatus          string             `json:"run_status,omitempty"`
	AccountSummaryID   string             `json:"account_summary_id,omitempty"`
	AccountStatus      string             `json:"account_status,omitempty"`
	AccountValidation  string             `json:"account_validation_status,omitempty"`
	FunnelFallback     string             `json:"funnel_fallback_reason,omitempty"`
	FunnelRepairable   []string           `json:"funnel_repairable_errors,omitempty"`
	CategoryRows       []CategoryDebugRow `json:"category_rows,omitempty"`
	RecommendedActions RecommendedDebug   `json:"recommended_actions,omitempty"`
}

type CategoryDebugRow struct {
	ScopeID          string   `json:"scope_id"`
	CategoryName     string   `json:"category_name,omitempty"`
	Status           string   `json:"status"`
	ValidationStatus string   `json:"validation_status,omitempty"`
	RepairableErrors []string `json:"repairable_errors,omitempty"`
	FallbackReason   string   `json:"fallback_reason,omitempty"`
	Accepted         bool     `json:"accepted"`
}

type RecommendedDebug struct {
	AIReceived   int    `json:"ai_actions_received"`
	AIAccepted   int    `json:"ai_actions_accepted"`
	Dropped      int    `json:"dropped_actions"`
	FallbackNote string `json:"fallback_note,omitempty"`
}

type AccountSummaryWidget struct {
	WidgetBase
	HealthScore          *int32                `json:"health_score,omitempty"`
	HealthScoreDelta     *int32                `json:"health_score_delta,omitempty"`
	Severity             string                `json:"severity,omitempty"`
	HealthStatusCode     string                `json:"health_status_code,omitempty"`
	HealthStatusLabel    string                `json:"health_status_label,omitempty"`
	HealthStatusExplain  string                `json:"health_status_explanation,omitempty"`
	Headline             string                `json:"headline,omitempty"`
	ExecutiveSummary     string                `json:"executive_summary,omitempty"`
	ImprovementSummary   string                `json:"improvement_summary,omitempty"`
	ImprovementLevers    []ImprovementLever    `json:"improvement_levers,omitempty"`
	RiskDrivers          []RiskDriver          `json:"risk_drivers,omitempty"`
	HealthScoreBreakdown *HealthScoreBreakdown `json:"health_score_breakdown,omitempty"`
	// HealthPolicyVersion/HealthFactsHash pin health_score to the backend
	// candidate-registry policy version and Dashboard Facts snapshot it was
	// computed from (see healthpolicy.HealthPolicyVersion). Populated when
	// the narrative was produced by the Unified publication adapter.
	HealthPolicyVersion string `json:"health_policy_version,omitempty"`
	HealthFactsHash     string `json:"health_facts_hash,omitempty"`
	MetricDate          string `json:"metric_date,omitempty"`
	// Freshness: current (matches dashboard metric_date), stale_metric_date (older AI run), stale_relative_to_ads (text conflicts with ad_efficiency).
	Freshness string `json:"freshness,omitempty"`
	// GeneratedAt is RFC3339 timestamp of the AI summary row when available.
	GeneratedAt string `json:"generated_at,omitempty"`
	// Optional debug/admin fields. Seller UI does not render them.
	NarrativeSource  string `json:"narrative_source,omitempty"`
	NarrativeRunID   string `json:"narrative_run_id,omitempty"`
	PromptVersion    string `json:"prompt_version,omitempty"`
	ValidationStatus string `json:"validation_status,omitempty"`
	// NarrativeStaleDebug is populated when DASHBOARD_NARRATIVE_DEBUG=1 (legacy alias).
	NarrativeStaleDebug *NarrativeStaleDebug `json:"narrative_stale_debug,omitempty"`
	// NarrativeDebug is populated when DASHBOARD_NARRATIVE_DEBUG=1.
	NarrativeDebug  *NarrativeDebug                `json:"narrative_debug,omitempty"`
	storedNarrative *storedDashboardNarrative      `json:"-"`
	narrativeBuild  *dashboardNarrativeBuildResult `json:"-"`
}

// ImprovementLever is one concrete way to improve the cabinet health index.
type ImprovementLever struct {
	Title             string `json:"title"`
	Explanation       string `json:"explanation"`
	FactorID          string `json:"factor_id,omitempty"`
	LinkedActionID    string `json:"linked_action_id,omitempty"`
	ExpectedDirection string `json:"expected_direction,omitempty"`
}

// PeriodContext describes value / comparison / trend scopes for the home page.
type PeriodContext struct {
	ValuePeriod      string `json:"value_period,omitempty"`
	ComparisonPeriod string `json:"comparison_period,omitempty"`
	TrendPeriod      string `json:"trend_period,omitempty"`
	CurrentRange     string `json:"current_range,omitempty"`
	PreviousRange    string `json:"previous_range,omitempty"`
	Caption          string `json:"caption,omitempty"`
	CaptionPrimary   string `json:"caption_primary,omitempty"`
	CaptionSecondary string `json:"caption_secondary,omitempty"`
}

type WhatChangedWidget struct {
	WidgetBase
	Summary        string             `json:"summary,omitempty"`
	MainConclusion string             `json:"main_conclusion,omitempty"`
	Groups         []WhatChangedGroup `json:"groups"`
}

type WhatChangedGroup struct {
	Key       string            `json:"key"`
	Label     string            `json:"label"`
	Tone      string            `json:"tone"`
	Summary   string            `json:"summary,omitempty"`
	Items     []WhatChangedItem `json:"items"`
	EmptyText string            `json:"empty_text,omitempty"`
}

type WhatChangedItem struct {
	Label             string   `json:"label"`
	Text              string   `json:"text"`
	Highlights        []string `json:"highlights,omitempty"`
	Domain            string   `json:"domain,omitempty"`
	Severity          string   `json:"severity,omitempty"`
	CurrentValueText  string   `json:"current_value_text,omitempty"`
	PreviousValueText string   `json:"previous_value_text,omitempty"`
	DeltaText         string   `json:"delta_text,omitempty"`
	ImpactText        string   `json:"impact_text,omitempty"`
	EntityCount       int      `json:"entity_count,omitempty"`
	EntityNames       []string `json:"entity_names,omitempty"`
	LinkedActionID    string   `json:"linked_action_id,omitempty"`
	ActionLinkLabel   string   `json:"action_link_label,omitempty"`
	DeepLink          string   `json:"deep_link,omitempty"`
	ChangeID          string   `json:"change_id,omitempty"`
}

type CategorySummaryCard struct {
	CategoryID        string `json:"category_id,omitempty"`
	CategoryName      string `json:"category_name"`
	CategoryScore     *int32 `json:"category_score,omitempty"`
	ScoreMax          int32  `json:"score_max,omitempty"`
	HealthScore       *int32 `json:"health_score,omitempty"`
	Severity          string `json:"severity,omitempty"`
	Headline          string `json:"headline,omitempty"`
	KeyRisk           string `json:"key_risk,omitempty"`
	RecommendedAction string `json:"recommended_action,omitempty"`
}

type FunnelStage struct {
	Key                    string   `json:"key"`
	Label                  string   `json:"label"`
	Value                  *int64   `json:"value,omitempty"`
	ConversionFromPrevious *float64 `json:"conversion_from_previous,omitempty"`
	ConversionFromTop      *float64 `json:"conversion_from_top,omitempty"`
	PreviousDayValue       *int64   `json:"previous_day_value"`
	ChangeVsPreviousDayPct *float64 `json:"change_vs_previous_day_pct"`
	// PreviousConversionFromPrevious is the prior-window conversion from the
	// previous funnel stage (same formula as ConversionFromPrevious).
	PreviousConversionFromPrevious *float64 `json:"previous_conversion_from_previous"`
	// ConversionDeltaPp is current − previous conversion in percentage points.
	ConversionDeltaPp *float64 `json:"conversion_delta_pp"`
	// PeriodType documents that Value is one completed day (D-1).
	PeriodType string `json:"period_type,omitempty"`
}

type FunnelDailyPoint struct {
	Date           string `json:"date"`
	Impressions    *int64 `json:"impressions"`
	ProductViews   *int64 `json:"product_views"`
	AddToCart      *int64 `json:"add_to_cart"`
	OrderedUnits   *int64 `json:"ordered_units"`
	DeliveredUnits *int64 `json:"delivered_units"`
}

type FunnelWeeklyStage struct {
	Key                            string   `json:"key"`
	Label                          string   `json:"label"`
	Value                          *int64   `json:"value,omitempty"`
	PreviousPeriodValue            *int64   `json:"previous_period_value"`
	ChangeVsPreviousPeriodPct      *float64 `json:"change_vs_previous_period_pct"`
	ConversionFromPrevious         *float64 `json:"conversion_from_previous,omitempty"`
	PreviousConversionFromPrevious *float64 `json:"previous_conversion_from_previous"`
	ConversionDeltaPp              *float64 `json:"conversion_delta_pp"`
}

type FunnelWeeklyWidget struct {
	WidgetBase
	Stages []FunnelWeeklyStage `json:"stages,omitempty"`
}

type SalesFunnelWidget struct {
	WidgetBase
	CurrentCompletedDate  string             `json:"current_completed_date"`
	PreviousCompletedDate string             `json:"previous_completed_date"`
	Stages                []FunnelStage      `json:"stages,omitempty"`
	DailySeries           []FunnelDailyPoint `json:"daily_series"`
	Overview7D            FunnelWeeklyWidget `json:"overview_7d"`
}

type FunnelAnalysisWidget struct {
	WidgetBase
	PrimaryDropStage   string                 `json:"primary_drop_stage,omitempty"`
	PrimaryDropMessage string                 `json:"primary_drop_message,omitempty"`
	AIAnalysis         FunnelAIAnalysisWidget `json:"ai_analysis"`
}

type FunnelAIAnalysisWidget struct {
	WidgetBase
	ShortSummary     string   `json:"short_summary,omitempty"`
	TrendVsYesterday string   `json:"trend_vs_yesterday,omitempty"`
	TrendVs7D        string   `json:"trend_vs_7d,omitempty"`
	RelatedSignals   []string `json:"related_signals,omitempty"`
	RecommendedFocus []string `json:"recommended_focus,omitempty"`
	FullAnalysis     string   `json:"full_analysis,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	Details          string   `json:"details,omitempty"`

	WeakestStage          FunnelWeakestStage          `json:"weakest_stage,omitempty"`
	MainConclusion        string                      `json:"main_conclusion,omitempty"`
	ActionItems           []string                    `json:"action_items,omitempty"`
	Comparisons           FunnelComparisons           `json:"comparisons,omitempty"`
	ComparisonsCommentary FunnelComparisonsCommentary `json:"comparisons_commentary,omitempty"`
	DropFactors           []FunnelDropFactor          `json:"drop_factors,omitempty"`
	CheckDirections       []FunnelTextItem            `json:"check_directions,omitempty"`
	DetailsSections       []FunnelDetailsSection      `json:"details_sections,omitempty"`
	// Legacy; populated from drop_factors when needed for older clients.
	InfluenceFactors []FunnelTextItem `json:"influence_factors,omitempty"`
}

type FunnelWeakestStage struct {
	Status        string `json:"status,omitempty"`
	Key           string `json:"key,omitempty"`
	Label         string `json:"label,omitempty"`
	Metric        string `json:"metric,omitempty"`
	Value         string `json:"value,omitempty"`
	Explanation   string `json:"explanation,omitempty"`
	Reason        string `json:"reason,omitempty"`
	DeltaPrevious string `json:"delta_previous,omitempty"`
	Delta7D       string `json:"delta_7d,omitempty"`
	Tone          string `json:"tone,omitempty"`
}

type FunnelComparisonsCommentary struct {
	PreviousPeriod  string `json:"previous_period,omitempty"`
	SevenDayAverage string `json:"seven_day_average,omitempty"`
}

type FunnelDropFactor struct {
	Key   string `json:"key,omitempty"`
	Label string `json:"label,omitempty"`
	// Semantic group for UI: funnel_signal | business_signal | possible_amplifier | context_hypothesis.
	// Set deterministically by key in applyFunnelAIGuards; optional in AI payload.
	Group         string `json:"group,omitempty"`
	EvidenceLevel string `json:"evidence_level,omitempty"`
	Metric        string `json:"metric,omitempty"`
	Delta         string `json:"delta,omitempty"`
	Explanation   string `json:"explanation,omitempty"`
	Limitation    string `json:"limitation,omitempty"`
	Tone          string `json:"tone,omitempty"`
	EvidenceKey   string `json:"evidence_key,omitempty"`
}

type FunnelDetailsSectionItem struct {
	Text    string   `json:"text,omitempty"`
	Metrics []string `json:"metrics,omitempty"`
}

type FunnelDetailsSection struct {
	Title string                     `json:"title,omitempty"`
	Items []FunnelDetailsSectionItem `json:"items,omitempty"`
}

type FunnelComparisonItem struct {
	Key   string `json:"key,omitempty"`
	Label string `json:"label,omitempty"`
	Value string `json:"value,omitempty"`
	Delta string `json:"delta,omitempty"`
	Tone  string `json:"tone,omitempty"`
}

type FunnelComparisons struct {
	PreviousPeriod    []FunnelComparisonItem `json:"previous_period,omitempty"`
	SevenDayAverage   []FunnelComparisonItem `json:"seven_day_average,omitempty"`
	StageMetrics      []FunnelComparisonItem `json:"stage_metrics,omitempty"`
	ConversionMetrics []FunnelComparisonItem `json:"conversion_metrics,omitempty"`
	BusinessMetrics   []FunnelComparisonItem `json:"business_metrics,omitempty"`
	ContextSignals    []FunnelComparisonItem `json:"context_signals,omitempty"`
}

type FunnelTextItem struct {
	Key        string `json:"key,omitempty"`
	Label      string `json:"label,omitempty"`
	Priority   string `json:"priority,omitempty"`
	DataNeeded string `json:"data_needed,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Text       string `json:"text,omitempty"`
	Tone       string `json:"tone,omitempty"`
}

type StockCoverageSegment struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	SKUCount int    `json:"sku_count"`
	Severity string `json:"severity"`
}

type StockCoverageWidget struct {
	WidgetBase
	Segments         []StockCoverageSegment     `json:"segments,omitempty"`
	TotalSKUs        int                        `json:"total_skus,omitempty"`
	SlowTurnoverLink *StockCoverageSKUFocusLink `json:"slow_turnover_link,omitempty"`
	Narrative        *StockCoverageNarrative    `json:"stock_coverage_narrative,omitempty"`
}

// StockCoverageNarrative is seller-facing stock analytics (factual metrics + optional AI interpretation).
type StockCoverageNarrative struct {
	Status           string                          `json:"status"`
	Headline         string                          `json:"headline,omitempty"`
	Summary          string                          `json:"summary,omitempty"`
	NextStep         string                          `json:"next_step,omitempty"`
	Tone             string                          `json:"tone,omitempty"`
	Source           string                          `json:"source,omitempty"`
	NoSalesInsight   string                          `json:"no_sales_insight,omitempty"`
	RiskComment      string                          `json:"risk_comment,omitempty"`
	Actions          []StockCoverageAction           `json:"actions,omitempty"`
	Checklist        []StockCoverageAction           `json:"checklist,omitempty"`
	RevenueAtRisk    StockCoverageNarrativeRevenue   `json:"revenue_at_risk"`
	Segments         []StockCoverageNarrativeSegment `json:"segments,omitempty"`
	RiskExamples     []StockCoverageRiskExample      `json:"risk_examples,omitempty"`
	NoSalesWithStock StockCoverageNoSalesWithStock   `json:"no_sales_with_stock"`
	AdvertisingLink  StockCoverageAdvertisingLink    `json:"advertising_link,omitempty"`
	SKUFocusLink     StockCoverageSKUFocusLink       `json:"sku_focus_link"`
	NoSalesFocusLink *StockCoverageSKUFocusLink      `json:"no_sales_focus_link,omitempty"`
}

type StockCoverageAction struct {
	Title    string                     `json:"title"`
	Text     string                     `json:"text,omitempty"`
	Priority string                     `json:"priority,omitempty"`
	Target   *StockCoverageActionTarget `json:"target,omitempty"`
}

type StockCoverageActionTarget struct {
	Type string `json:"type"`
	Tab  string `json:"tab,omitempty"`
}

type StockCoverageNarrativeRevenue struct {
	Status                 string   `json:"status"`
	Label                  string   `json:"label,omitempty"`
	Value                  string   `json:"value,omitempty"`
	MethodNote             string   `json:"method_note,omitempty"`
	ShareOfDailyRevenuePct *float64 `json:"share_of_daily_revenue_pct,omitempty"`
}

type StockCoverageNarrativeSegment struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Count    int    `json:"count"`
	Severity string `json:"severity"`
	Comment  string `json:"comment,omitempty"`
}

type StockCoverageRiskExample struct {
	ProductName   string `json:"product_name"`
	StockLabel    string `json:"stock_label"`
	CoverageLabel string `json:"coverage_label"`
	RiskLabel     string `json:"risk_label,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type StockCoverageNoSalesWithStock struct {
	Status     string `json:"status"`
	Count      int    `json:"count,omitempty"`
	ShownCount int    `json:"shown_count,omitempty"`
	StockUnits int    `json:"stock_units,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Comment    string `json:"comment,omitempty"`
	Target     string `json:"target,omitempty"`
}

type StockCoverageAdvertisingLink struct {
	Status  string `json:"status"`
	Comment string `json:"comment,omitempty"`
}

type StockCoverageSKUFocusLink struct {
	Label  string `json:"label"`
	Target string `json:"target"`
}

type AdEfficiencyPoint struct {
	CampaignExternalID int64    `json:"campaign_external_id"`
	CampaignName       string   `json:"campaign_name"`
	Spend              float64  `json:"spend"`
	DRR                *float64 `json:"drr,omitempty"`
	Revenue            float64  `json:"revenue"`
	Orders             int64    `json:"orders"`
	EfficiencySignal   string   `json:"efficiency_signal"`
}

type AdEfficiencyPeriod struct {
	MetricDate            string `json:"metric_date"`
	DateFrom              string `json:"date_from"`
	DateTo                string `json:"date_to"`
	CurrentCompletedDate  string `json:"current_completed_date"`
	PreviousCompletedDate string `json:"previous_completed_date"`
	Current7dDateFrom     string `json:"current_7d_date_from"`
	Current7dDateTo       string `json:"current_7d_date_to"`
	Previous7dDateFrom    string `json:"previous_7d_date_from"`
	Previous7dDateTo      string `json:"previous_7d_date_to"`
	SampleDays            int    `json:"sample_days"`
	PreviousSampleDays    int    `json:"previous_sample_days"`
}

type AdEfficiencySummary struct {
	// CurrentDRRPct / AverageDRR7dPct = ad spend share of attributed ad revenue
	// (NOT store revenue). Seller Pulse uses store-revenue share separately.
	CurrentDRRPct       *float64 `json:"current_drr_pct,omitempty"`
	PreviousDRRPct      *float64 `json:"previous_drr_pct,omitempty"`
	AverageDRR7dPct     *float64 `json:"average_drr_7d_pct,omitempty"`
	DeltaVsPreviousPp   *float64 `json:"delta_vs_previous_pp,omitempty"`
	DeltaVsAverage7dPp  *float64 `json:"delta_vs_average_7d_pp,omitempty"`
	PreviousDRR7dPct    *float64 `json:"previous_drr_7d_pct,omitempty"`
	DeltaVsPrevious7dPp *float64 `json:"delta_vs_previous_7d_pp,omitempty"`

	TotalSpendToday          float64 `json:"total_spend_today"`
	TotalRevenueToday        float64 `json:"total_revenue_today"` // attributed ad revenue today
	TotalStoreRevenueToday   float64 `json:"total_store_revenue_today"`
	TotalOrdersToday         int64   `json:"total_orders_today"`
	TotalSpend7d             float64 `json:"total_spend_7d"`
	TotalAttributedRevenue7d float64 `json:"total_attributed_revenue_7d"`
	TotalStoreRevenue7d      float64 `json:"total_store_revenue_7d"`

	CampaignsHighDRRCount  int     `json:"campaigns_high_drr_count"`
	CampaignsNoOrdersCount int     `json:"campaigns_no_orders_count"`
	HighDRRSpend           float64 `json:"high_drr_spend"`

	SpendWithoutOrders         float64 `json:"spend_without_orders"`
	SpendWithoutOrdersDateFrom string  `json:"spend_without_orders_date_from,omitempty"`
	SpendWithoutOrdersDateTo   string  `json:"spend_without_orders_date_to,omitempty"`

	HighDRRThresholdPct float64 `json:"high_drr_threshold_pct"`
}

type AdEfficiencyTrendPoint struct {
	Date        string   `json:"date"`
	Label       string   `json:"label,omitempty"`
	Spend       float64  `json:"spend"`
	Revenue     float64  `json:"revenue"`
	Orders      int64    `json:"orders"`
	DRRPct      *float64 `json:"drr_pct,omitempty"`
	IsToday     bool     `json:"is_today,omitempty"`
	IsYesterday bool     `json:"is_yesterday,omitempty"`
	DataStatus  string   `json:"data_status"`
}

type AdCampaignDisplayRow struct {
	CampaignExternalID int64    `json:"campaign_external_id"`
	CampaignName       string   `json:"campaign_name"`
	DRRPct             *float64 `json:"drr_pct,omitempty"`
	Spend              float64  `json:"spend"`
	Orders             int64    `json:"orders"`
	Revenue            float64  `json:"revenue"`
	ROAS               *float64 `json:"roas,omitempty"`
	CPO                *float64 `json:"cpo,omitempty"`
	CTR                *float64 `json:"ctr,omitempty"`
	CPC                *float64 `json:"cpc,omitempty"`
	ClickToOrderRate   *float64 `json:"click_to_order_rate,omitempty"`
	ReasonCode         string   `json:"reason_code"`
}

type AdEfficiencyNarrative struct {
	Status         string                  `json:"status"`
	Headline       string                  `json:"headline,omitempty"`
	Summary        string                  `json:"summary,omitempty"`
	Recommendation string                  `json:"recommendation,omitempty"`
	Checklist      []AdEfficiencyCheckItem `json:"checklist,omitempty"`
	Limitations    []string                `json:"limitations,omitempty"`
	// MetricClaims / ThresholdClaims are provenance for bundle validation and
	// audit/replay. Copied from validated Unified output; not required for
	// seller-facing UI rendering.
	MetricClaims    []AdEfficiencyMetricClaim    `json:"metric_claims,omitempty"`
	ThresholdClaims []AdEfficiencyThresholdClaim `json:"threshold_claims,omitempty"`
}

// AdEfficiencyMetricClaim is a Pack-grounded numeric assertion cited in the
// advertising narrative (mirrors contract.UnifiedMetricClaimV1).
type AdEfficiencyMetricClaim struct {
	FactID      string   `json:"fact_id"`
	MetricCode  string   `json:"metric_code"`
	Value       *float64 `json:"value"`
	PeriodType  string   `json:"period_type"`
	PeriodStart string   `json:"period_start"`
	PeriodEnd   string   `json:"period_end"`
}

// AdEfficiencyThresholdClaim is a policy-backed comparative threshold cited
// in advertising narrative (mirrors contract.UnifiedThresholdClaimV1).
type AdEfficiencyThresholdClaim struct {
	PolicyID   string  `json:"policy_id"`
	MetricCode string  `json:"metric_code"`
	Scope      string  `json:"scope"`
	Operator   string  `json:"operator"`
	Value      float64 `json:"value"`
}

type AdEfficiencyCheckItem struct {
	Title                string   `json:"title"`
	Text                 string   `json:"text,omitempty"`
	Priority             string   `json:"priority,omitempty"`
	Target               string   `json:"target,omitempty"`
	SupportingMetricRefs []string `json:"supporting_metric_refs,omitempty"`
}

type AdEfficiencyCTA struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

type AdEfficiencyWidget struct {
	WidgetBase
	Period           *AdEfficiencyPeriod      `json:"period,omitempty"`
	Summary          *AdEfficiencySummary     `json:"summary,omitempty"`
	Trend7D          []AdEfficiencyTrendPoint `json:"trend_7d,omitempty"`
	HighDRRCampaigns []AdCampaignDisplayRow   `json:"high_drr_campaigns,omitempty"`
	NoOrderCampaigns []AdCampaignDisplayRow   `json:"no_order_campaigns,omitempty"`
	Narrative        *AdEfficiencyNarrative   `json:"ad_efficiency_narrative,omitempty"`
	CTA              *AdEfficiencyCTA         `json:"cta,omitempty"`
	// Legacy only — scatter plot backward compatibility; do not extend.
	Points []AdEfficiencyPoint `json:"points,omitempty"`
}

type RiskGroupCount struct {
	Group    string `json:"group"`
	Label    string `json:"label"`
	Count    int64  `json:"count"`
	Severity string `json:"severity"`
}

type RiskDistributionWidget struct {
	WidgetBase
	Groups []RiskGroupCount `json:"groups,omitempty"`
	Total  int64            `json:"total,omitempty"`
}

type SellerPulse struct {
	KPIs      []KPICard             `json:"kpis"`
	Trend7D   []TrendDayPoint       `json:"trend_7d"`
	Narrative *SellerPulseNarrative `json:"narrative,omitempty"`
}

// SellerPulseNarrative is seller-facing interpretive copy for the pulse block (AI-validated or deterministic fallback).
type SellerPulseNarrative struct {
	Status          string `json:"status"`
	Tone            string `json:"tone,omitempty"`
	Headline        string `json:"headline"`
	DisplayHeadline bool   `json:"display_headline,omitempty"`
	Summary         string `json:"summary,omitempty"`
	// KeyPoints are AI-резюме bullets (separate from ImportantNotes).
	KeyPoints []SellerPulseImportantNote `json:"key_points,omitempty"`
	// ImportantNotes populate «Что важно знать».
	ImportantNotes  []SellerPulseImportantNote `json:"important_notes,omitempty"`
	QuickActions    []SellerPulseQuickAction   `json:"quick_actions,omitempty"`
	Chips           []SellerPulseChip          `json:"chips,omitempty"`
	DataQualityNote string                     `json:"data_quality_note,omitempty"`
}

type SellerPulseImportantNote struct {
	Title      string   `json:"title,omitempty"`
	Text       string   `json:"text"`
	Tone       string   `json:"tone,omitempty"`
	MetricRefs []string `json:"metric_refs,omitempty"`
	FactIDs    []string `json:"fact_ids,omitempty"`
	EntityIDs  []string `json:"entity_ids,omitempty"`
}

type SellerPulseQuickAction struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Href        string `json:"href,omitempty"`
	ActionID    int64  `json:"action_id,omitempty"`
	ActionKey   string `json:"action_key,omitempty"`
}

type SellerPulseChip struct {
	Label string `json:"label"`
	Tone  string `json:"tone"`
}

type KPICard struct {
	Key         string           `json:"key"`
	Label       string           `json:"label"`
	Value       string           `json:"value"`
	Delta       *KPICardDelta    `json:"delta,omitempty"`
	Sparkline7D []SparklinePoint `json:"sparkline_7d,omitempty"`
	Trend       *KPICardTrend    `json:"trend,omitempty"`
	Hint        string           `json:"hint,omitempty"`
}

type KPICardTrend struct {
	Status string `json:"status,omitempty"`
	Label  string `json:"label,omitempty"`
}

type KPICardDelta struct {
	Abs     float64  `json:"abs"`
	Pct     *float64 `json:"pct,omitempty"`
	Label   string   `json:"label"`
	Unit    string   `json:"unit,omitempty"`
	Display string   `json:"display,omitempty"`
}

type SparklinePoint struct {
	Date    string   `json:"date"`
	Value   *float64 `json:"value,omitempty"`
	Revenue *float64 `json:"revenue,omitempty"`
	HasData bool     `json:"has_data"`
}

type TrendDayPoint struct {
	Date    string   `json:"date"`
	Revenue *float64 `json:"revenue,omitempty"`
	Orders  *int32   `json:"orders,omitempty"`
}

type SKUFocus struct {
	OrderDrop                  []SKUFocusCard `json:"order_drop"`
	StockRisk                  []SKUFocusCard `json:"stock_risk"`
	StockRiskTotalCount        int            `json:"stock_risk_total_count,omitempty"`
	NoSalesWithStock           []SKUFocusCard `json:"no_sales_with_stock"`
	NoSalesWithStockTotalCount int            `json:"no_sales_with_stock_total_count,omitempty"`
	Surplus                    []SKUFocusCard `json:"surplus,omitempty"` // deprecated: omitted in seller-facing responses; use slow_turnover
	SlowTurnover               []SKUFocusCard `json:"slow_turnover,omitempty"`
	KeySKUs                    []KeySkuCard   `json:"key_skus"`
	KeySKUsTotalCount          int            `json:"key_skus_total_count,omitempty"`
	StoreTotalRevenue7d        float64        `json:"store_total_revenue_7d,omitempty"`    // sum of revenue_7d across all SKUs with sales in the 7-day window
	KeySKUsTotalRevenue7d      float64        `json:"key_skus_total_revenue_7d,omitempty"` // sum of revenue_7d for SKUs in the key cohort
}

type SKUFocusCard struct {
	OzonProductID int64   `json:"ozon_product_id"`
	SKU           *int64  `json:"sku,omitempty"`
	OfferID       *string `json:"offer_id,omitempty"`
	ProductName   string  `json:"product_name"`
	ImageURL      *string `json:"image_url,omitempty"`
	Problem       string  `json:"problem"`
	KeyMetric     string  `json:"key_metric"`
	// TrendStatus is available | not_enough_data | source_not_available.
	// Sparse sparklines never drop the card; they set not_enough_data.
	TrendStatus        string            `json:"trend_status,omitempty"`
	TrendMessage       string            `json:"trend_message,omitempty"`
	Sparkline7D        []SparklinePoint  `json:"sparkline_7d,omitempty"`
	StockTrend7D       []StockTrendPoint `json:"stock_trend_7d,omitempty"`
	ProductURL         *string           `json:"product_url,omitempty"`
	CurrentStock       *int32            `json:"current_stock,omitempty"`
	PreviousStock      *int32            `json:"previous_stock,omitempty"`
	StockDeltaDayToDay *int32            `json:"stock_delta_day_to_day,omitempty"`
	CoverageDays       *float64          `json:"coverage_days,omitempty"`
	// CoverageBucket is the canonical stockpolicy segment (e.g. critical_0_3).
	// Frontend filters must use this field and must not recompute boundaries.
	CoverageBucket          string   `json:"coverage_bucket,omitempty"`
	AvgDailyRevenue7d       *float64 `json:"avg_daily_revenue_7d,omitempty"`
	RevenueAtRisk7d         *float64 `json:"revenue_at_risk_7d,omitempty"`
	OrdersDeltaDayToDay     *int32   `json:"orders_delta_day_to_day,omitempty"`
	RevenueDeltaDayToDay    *float64 `json:"revenue_delta_day_to_day,omitempty"`
	PreviousOrders          *int32   `json:"previous_orders,omitempty"`
	CurrentOrders           *int32   `json:"current_orders,omitempty"`
	NoSalesDays             *int32   `json:"no_sales_days,omitempty"`
	LastCompletedMetricDate *string  `json:"last_completed_metric_date,omitempty"`
	UnitPrice               *float64 `json:"unit_price,omitempty"`
	FrozenStockValue        *float64 `json:"frozen_stock_value,omitempty"`
	PreviousDayOrders       *int32   `json:"previous_day_orders,omitempty"`
}

// SKU focus trend status vocabulary (frozen publication contract).
const (
	SKUFocusTrendAvailable          = "available"
	SKUFocusTrendNotEnoughData      = "not_enough_data"
	SKUFocusTrendSourceNotAvailable = "source_not_available"
)

type RiskDriver struct {
	ID                      string         `json:"id"`
	Category                string         `json:"category"`
	CategoryLabel           string         `json:"category_label"`
	Severity                string         `json:"severity"`
	ImpactScore             int            `json:"impact_score"`
	Title                   string         `json:"title"`
	Reason                  string         `json:"reason"`
	Evidence                map[string]any `json:"evidence,omitempty"`
	LinkedAlertIDs          []int64        `json:"linked_alert_ids,omitempty"`
	LinkedRecommendationIDs []int64        `json:"linked_recommendation_ids,omitempty"`
}

type HealthScoreBreakdownItem struct {
	DriverID            string  `json:"driver_id"`
	Category            string  `json:"category"`
	Severity            string  `json:"severity"`
	ImpactScore         int     `json:"impact_score"`
	Weight              float64 `json:"weight"`
	SeverityBase        float64 `json:"severity_base"`
	ImpactPart          float64 `json:"impact_part"`
	RawDriverPenalty    float64 `json:"raw_driver_penalty"`
	CappedDriverPenalty float64 `json:"capped_driver_penalty"`
}

type HealthScoreBreakdown struct {
	Drivers          []HealthScoreBreakdownItem `json:"drivers"`
	RawPenalty       float64                    `json:"raw_penalty"`
	SaturatedPenalty float64                    `json:"saturated_penalty"`
	FinalScore       int32                      `json:"final_score"`
}

type TopActionSupportingMetric struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type TopActionItem struct {
	ID                      string                      `json:"id"`
	ActionID                int64                       `json:"action_id,omitempty"`
	ActionKey               string                      `json:"action_key,omitempty"`
	ActionsRunID            int64                       `json:"actions_run_id,omitempty"`
	PipelineRunID           int64                       `json:"pipeline_run_id,omitempty"`
	Type                    string                      `json:"type,omitempty"`
	Summary                 string                      `json:"summary,omitempty"`
	CompletionCriteria      string                      `json:"completion_criteria,omitempty"`
	VisibleLimitations      []string                    `json:"visible_limitations,omitempty"`
	DriverID                string                      `json:"driver_id,omitempty"`
	Source                  string                      `json:"source"`
	Priority                string                      `json:"priority"`
	PriorityLabel           string                      `json:"priority_label"`
	Category                string                      `json:"category"`
	CategoryLabel           string                      `json:"category_label"`
	Title                   string                      `json:"title"`
	Reason                  string                      `json:"reason"`
	NextStep                string                      `json:"next_step"`
	ExpectedEffect          string                      `json:"expected_effect,omitempty"`
	LinkedAlertIDs          []int64                     `json:"linked_alert_ids,omitempty"`
	LinkedRecommendationIDs []int64                     `json:"linked_recommendation_ids,omitempty"`
	SupportingMetrics       []TopActionSupportingMetric `json:"supporting_metrics,omitempty"`
}

type ActionItem struct {
	Priority               string  `json:"priority"`
	UrgencyGroup           string  `json:"urgency_group"`
	Action                 string  `json:"action"`
	Reason                 string  `json:"reason,omitempty"`
	ExpectedEffect         string  `json:"expected_effect,omitempty"`
	RelatedEntityType      string  `json:"related_entity_type,omitempty"`
	RelatedEntityID        *string `json:"related_entity_id,omitempty"`
	Source                 string  `json:"source"`
	LinkedAlertID          *int64  `json:"linked_alert_id,omitempty"`
	LinkedRecommendationID *int64  `json:"linked_recommendation_id,omitempty"`
}
