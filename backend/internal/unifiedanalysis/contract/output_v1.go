package contract

// UnifiedAnalysisOutputV1 is the AI-facing unified analysis response contract.
// It intentionally excludes DB IDs, pipeline run IDs, publication timestamps,
// user state, CTAs, debug blocks, provider metadata, legacy candidate IDs,
// evidence fact IDs, and global context hashes — backend adds those later.
type UnifiedAnalysisOutputV1 struct {
	SchemaVersion   string                `json:"schema_version"`
	AnalysisMeta    UnifiedAnalysisMetaV1 `json:"analysis_meta"`
	AnalysisOutcome string                `json:"analysis_outcome"`
	Dashboard       UnifiedDashboardV1    `json:"dashboard"`
	Actions         []UnifiedActionV1     `json:"actions"`
	Limitations     []string              `json:"limitations"`
}

// UnifiedAnalysisMetaV1 references the Analysis Pack used for the run.
type UnifiedAnalysisMetaV1 struct {
	PackID     string `json:"pack_id"`
	PackHash   string `json:"pack_hash"`
	MetricDate string `json:"metric_date"`
}

// UnifiedDashboardV1 carries AI-facing dashboard content without runtime/debug fields.
type UnifiedDashboardV1 struct {
	AccountSummary    UnifiedAccountSummaryV1    `json:"account_summary"`
	WhatChanged       UnifiedWhatChangedV1       `json:"what_changed"`
	CategorySummaries []UnifiedCategorySummaryV1 `json:"category_summaries"`
	SalesFunnel       UnifiedSalesFunnelV1       `json:"sales_funnel"`
	FunnelAnalysis    UnifiedFunnelAnalysisV1    `json:"funnel_analysis"`
	StockCoverage     UnifiedStockCoverageV1     `json:"stock_coverage"`
	AdEfficiency      UnifiedAdEfficiencyV1      `json:"ad_efficiency"`
	RiskDistribution  UnifiedRiskDistributionV1  `json:"risk_distribution"`
	SellerPulse       UnifiedSellerPulseV1       `json:"seller_pulse"`
	SKUFocus          UnifiedSKUFocusV1          `json:"sku_focus"`

	TopActionIDs         []string `json:"top_action_ids"`
	RecommendedActionIDs []string `json:"recommended_action_ids"`
}

// UnifiedWidgetBase is shared availability metadata for dashboard blocks.
type UnifiedWidgetBase struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type UnifiedAccountSummaryV1 struct {
	UnifiedWidgetBase
	HealthScore        *int32                      `json:"health_score,omitempty"`
	HealthScoreDelta   *int32                      `json:"health_score_delta,omitempty"`
	Severity           string                      `json:"severity,omitempty"`
	Headline           string                      `json:"headline,omitempty"`
	ExecutiveSummary   string                      `json:"executive_summary,omitempty"`
	ImprovementSummary string                      `json:"improvement_summary,omitempty"`
	ImprovementLevers  []UnifiedImprovementLeverV2 `json:"improvement_levers,omitempty"`
	RiskDrivers        []UnifiedRiskDriverV1       `json:"risk_drivers,omitempty"`
	MetricDate         string                      `json:"metric_date,omitempty"`
}

type UnifiedImprovementLeverV2 struct {
	Title             string `json:"title"`
	Explanation       string `json:"explanation"`
	FactorID          string `json:"factor_id"`
	LinkedActionID    string `json:"linked_action_id,omitempty"`
	ExpectedDirection string `json:"expected_direction"`
}

type UnifiedRiskDriverV1 struct {
	ID             string   `json:"id"`
	Category       string   `json:"category"`
	CategoryLabel  string   `json:"category_label"`
	Severity       string   `json:"severity"`
	ImpactScore    int      `json:"impact_score"`
	Title          string   `json:"title"`
	Reason         string   `json:"reason"`
	FactIDs        []string `json:"fact_ids,omitempty"`
	EntityIDs      []string `json:"entity_ids,omitempty"`
	ImpactLevel    string   `json:"impact_level,omitempty"`
	AffectedMetric string   `json:"affected_metric,omitempty"`
}

type UnifiedWhatChangedV1 struct {
	UnifiedWidgetBase
	Summary        string                      `json:"summary,omitempty"`
	MainConclusion string                      `json:"main_conclusion,omitempty"`
	Groups         []UnifiedWhatChangedGroupV1 `json:"groups"`
}

type UnifiedWhatChangedGroupV1 struct {
	Key       string                     `json:"key"`
	Label     string                     `json:"label"`
	Tone      string                     `json:"tone"`
	Summary   string                     `json:"summary,omitempty"`
	Items     []UnifiedWhatChangedItemV1 `json:"items"`
	EmptyText string                     `json:"empty_text,omitempty"`
}

type UnifiedWhatChangedItemV1 struct {
	Label                  string   `json:"label"`
	Text                   string   `json:"text"`
	Highlights             []string `json:"highlights,omitempty"`
	SignalID               string   `json:"signal_id,omitempty"`
	SignalType             string   `json:"signal_type,omitempty"`
	PrimarySection         string   `json:"primary_section,omitempty"`
	FactIDs                []string `json:"fact_ids,omitempty"`
	EntityIDs              []string `json:"entity_ids,omitempty"`
	Metric                 string   `json:"metric,omitempty"`
	Direction              string   `json:"direction,omitempty"`
	Period                 string   `json:"period,omitempty"`
	Severity               string   `json:"severity,omitempty"`
	PreviousState          string   `json:"previous_state,omitempty"`
	CurrentState           string   `json:"current_state,omitempty"`
	FirstSeenPublicationID string   `json:"first_seen_publication_id,omitempty"`
	// Rich weekly-change fields (weeklychangepolicy / projection).
	Domain            string   `json:"domain,omitempty"`
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

type UnifiedCategorySummaryV1 struct {
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

type UnifiedFunnelStageV1 struct {
	Key                    string   `json:"key"`
	Label                  string   `json:"label"`
	Value                  *int64   `json:"value,omitempty"`
	ConversionFromPrevious *float64 `json:"conversion_from_previous,omitempty"`
	ConversionFromTop      *float64 `json:"conversion_from_top,omitempty"`
	PreviousDayValue       *int64   `json:"previous_day_value,omitempty"`
	ChangeVsPreviousDayPct *float64 `json:"change_vs_previous_day_pct,omitempty"`
}

type UnifiedSalesFunnelV1 struct {
	UnifiedWidgetBase
	Stages []UnifiedFunnelStageV1 `json:"stages,omitempty"`
}

type UnifiedFunnelAnalysisV1 struct {
	UnifiedWidgetBase
	PrimaryDropStage   string                    `json:"primary_drop_stage,omitempty"`
	PrimaryDropMessage string                    `json:"primary_drop_message,omitempty"`
	AIAnalysis         UnifiedFunnelAIAnalysisV1 `json:"ai_analysis"`
}

type UnifiedFunnelAIAnalysisV1 struct {
	UnifiedWidgetBase
	ShortSummary     string   `json:"short_summary,omitempty"`
	TrendVsYesterday string   `json:"trend_vs_yesterday,omitempty"`
	TrendVs7D        string   `json:"trend_vs_7d,omitempty"`
	RelatedSignals   []string `json:"related_signals,omitempty"`
	RecommendedFocus []string `json:"recommended_focus,omitempty"`
	FullAnalysis     string   `json:"full_analysis,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	Details          string   `json:"details,omitempty"`
	MainConclusion   string   `json:"main_conclusion,omitempty"`
	ActionItems      []string `json:"action_items,omitempty"`
}

type UnifiedStockCoverageSegmentV1 struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	SKUCount int    `json:"sku_count"`
	Severity string `json:"severity"`
}

type UnifiedStockCoverageNarrativeV1 struct {
	Status   string `json:"status"`
	Headline string `json:"headline,omitempty"`
	Summary  string `json:"summary,omitempty"`
	NextStep string `json:"next_step,omitempty"`
	Tone     string `json:"tone,omitempty"`
}

type UnifiedStockCoverageV1 struct {
	UnifiedWidgetBase
	Segments  []UnifiedStockCoverageSegmentV1  `json:"segments,omitempty"`
	TotalSKUs int                              `json:"total_skus,omitempty"`
	Narrative *UnifiedStockCoverageNarrativeV1 `json:"narrative,omitempty"`
}

type UnifiedAdEfficiencySummaryV1 struct {
	CurrentDRRPct          *float64 `json:"current_drr_pct,omitempty"`
	PreviousDRRPct         *float64 `json:"previous_drr_pct,omitempty"`
	AverageDRR7dPct        *float64 `json:"average_drr_7d_pct,omitempty"`
	TotalSpendToday        float64  `json:"total_spend_today"`
	TotalRevenueToday      float64  `json:"total_revenue_today"`
	TotalOrdersToday       int64    `json:"total_orders_today"`
	CampaignsHighDRRCount  int      `json:"campaigns_high_drr_count"`
	CampaignsNoOrdersCount int      `json:"campaigns_no_orders_count"`
	HighDRRSpend           float64  `json:"high_drr_spend"`
	SpendWithoutOrders     float64  `json:"spend_without_orders"`
}

type UnifiedAdEfficiencyNarrativeV1 struct {
	Status         string                               `json:"status"`
	Headline       string                               `json:"headline,omitempty"`
	Summary        string                               `json:"summary,omitempty"`
	Recommendation string                               `json:"recommendation,omitempty"`
	Checklist      []UnifiedAdEfficiencyChecklistItemV1 `json:"checklist,omitempty"`
	Limitations    []string                             `json:"limitations,omitempty"`
	// MetricClaims ground every free-text factual percentage in headline/summary
	// against explicit Pack facts (schema v9+). Required canonical
	// metric_code values: ad_spend_share_of_total_store_revenue_pct,
	// attributed_campaign_drr_pct, campaign_drr_pct, ad_spend,
	// store_total_revenue, attributed_ad_revenue, ad_orders.
	// outputvalidator blocks publication on a missing/mismatched claim; the
	// backend never rewrites the text.
	MetricClaims []UnifiedMetricClaimV1 `json:"metric_claims,omitempty"`
	// ThresholdClaims ground free-text comparative thresholds (schema v10+)
	// against the versioned business policy registry (e.g.
	// high_drr_campaign_threshold = campaign_drr_pct > 80). AI must copy
	// policy fields verbatim — inventing thresholds blocks publication.
	ThresholdClaims []UnifiedThresholdClaimV1 `json:"threshold_claims,omitempty"`
}

// UnifiedThresholdClaimV1 is one policy-backed comparative threshold cited in
// advertising (or related) narrative prose. It is NOT a Pack fact value.
type UnifiedThresholdClaimV1 struct {
	PolicyID   string  `json:"policy_id"`
	MetricCode string  `json:"metric_code"`
	Scope      string  `json:"scope"`    // campaign|account|sku
	Operator   string  `json:"operator"` // gt|gte|lt|lte|eq
	Value      float64 `json:"value"`
}

// UnifiedAdEfficiencyChecklistItemV1 is one AI-authored advertising checklist
// item (e.g. "review campaign X for high DRR"). The backend never invents
// checklist templates — the AI must emit them; validators only enforce
// presence/shape.
type UnifiedAdEfficiencyChecklistItemV1 struct {
	Title  string `json:"title"`
	Text   string `json:"text"`
	Target string `json:"target,omitempty"`
}

type UnifiedAdEfficiencyV1 struct {
	UnifiedWidgetBase
	Summary   *UnifiedAdEfficiencySummaryV1   `json:"summary,omitempty"`
	Narrative *UnifiedAdEfficiencyNarrativeV1 `json:"narrative,omitempty"`
}

type UnifiedRiskGroupCountV1 struct {
	Group    string `json:"group"`
	Label    string `json:"label"`
	Count    int64  `json:"count"`
	Severity string `json:"severity"`
}

type UnifiedRiskDistributionV1 struct {
	UnifiedWidgetBase
	Groups []UnifiedRiskGroupCountV1 `json:"groups,omitempty"`
	Total  int64                     `json:"total,omitempty"`
}

type UnifiedKPICardDeltaV1 struct {
	Abs   float64  `json:"abs"`
	Pct   *float64 `json:"pct,omitempty"`
	Label string   `json:"label"`
	Unit  string   `json:"unit,omitempty"`
}

type UnifiedKPICardV1 struct {
	Key   string                 `json:"key"`
	Label string                 `json:"label"`
	Value string                 `json:"value"`
	Delta *UnifiedKPICardDeltaV1 `json:"delta,omitempty"`
	Hint  string                 `json:"hint,omitempty"`
}

type UnifiedTrendDayPointV1 struct {
	Date    string   `json:"date"`
	Revenue *float64 `json:"revenue,omitempty"`
	Orders  *int32   `json:"orders,omitempty"`
}

type UnifiedSellerPulseNarrativeV1 struct {
	Status          string `json:"status"`
	Tone            string `json:"tone,omitempty"`
	Headline        string `json:"headline"`
	Summary         string `json:"summary,omitempty"`
	DataQualityNote string `json:"data_quality_note,omitempty"`
	// KeyPoints are short analytical bullets inside AI-резюме (not «Что важно знать»).
	KeyPoints []UnifiedSellerPulsePointV1 `json:"key_points,omitempty"`
	// ImportantFacts populate the separate «Что важно знать» card.
	ImportantFacts []UnifiedSellerPulsePointV1 `json:"important_facts,omitempty"`
	// MetricClaims ground numeric assertions in headline/summary against Pack facts.
	MetricClaims []UnifiedMetricClaimV1 `json:"metric_claims,omitempty"`
}

// UnifiedMetricClaimV1 is one numeric assertion with an explicit period and fact link.
type UnifiedMetricClaimV1 struct {
	FactID      string   `json:"fact_id"`
	MetricCode  string   `json:"metric_code"`
	Value       *float64 `json:"value"`
	PeriodType  string   `json:"period_type"` // current_day|previous_day|current_7d|previous_7d
	PeriodStart string   `json:"period_start"`
	PeriodEnd   string   `json:"period_end"`
}

// UnifiedSellerPulsePointV1 is a seller-pulse narrative bullet with optional structured claims.
type UnifiedSellerPulsePointV1 struct {
	UnifiedDiagnosticSignalV2
	MetricCode  string                 `json:"metric_code,omitempty"`
	Value       *float64               `json:"value,omitempty"`
	PeriodType  string                 `json:"period_type,omitempty"`
	PeriodStart string                 `json:"period_start,omitempty"`
	PeriodEnd   string                 `json:"period_end,omitempty"`
	Claims      []UnifiedMetricClaimV1 `json:"claims,omitempty"`
}

// UnifiedDiagnosticSignalV2 is one analytical signal with explicit ownership
// of a primary dashboard section (dedupe key across pulse / diagnostics).
type UnifiedDiagnosticSignalV2 struct {
	SignalID               string   `json:"signal_id"`
	SignalType             string   `json:"signal_type"`
	PrimarySection         string   `json:"primary_section"`
	Title                  string   `json:"title"`
	Explanation            string   `json:"explanation"`
	FactIDs                []string `json:"fact_ids,omitempty"`
	EntityIDs              []string `json:"entity_ids,omitempty"`
	Metric                 string   `json:"metric,omitempty"`
	Direction              string   `json:"direction,omitempty"`
	Period                 string   `json:"period,omitempty"`
	Severity               string   `json:"severity,omitempty"`
	PreviousState          string   `json:"previous_state,omitempty"`
	CurrentState           string   `json:"current_state,omitempty"`
	FirstSeenPublicationID string   `json:"first_seen_publication_id,omitempty"`
}

type UnifiedSellerPulseV1 struct {
	KPIs      []UnifiedKPICardV1             `json:"kpis"`
	Trend7D   []UnifiedTrendDayPointV1       `json:"trend_7d"`
	Narrative *UnifiedSellerPulseNarrativeV1 `json:"narrative,omitempty"`
}

type UnifiedSKUFocusCardV1 struct {
	SKU         *int64  `json:"sku,omitempty"`
	OfferID     *string `json:"offer_id,omitempty"`
	ProductName string  `json:"product_name"`
	Problem     string  `json:"problem"`
	KeyMetric   string  `json:"key_metric"`
}

type UnifiedKeySKUCardV1 struct {
	SKU             *int64   `json:"sku,omitempty"`
	OfferID         *string  `json:"offer_id,omitempty"`
	ProductName     string   `json:"product_name"`
	RevenueSharePct *float64 `json:"revenue_share_pct,omitempty"`
	Reason          string   `json:"reason,omitempty"`
}

type UnifiedSKUFocusV1 struct {
	OrderDrop        []UnifiedSKUFocusCardV1 `json:"order_drop"`
	StockRisk        []UnifiedSKUFocusCardV1 `json:"stock_risk"`
	NoSalesWithStock []UnifiedSKUFocusCardV1 `json:"no_sales_with_stock"`
	SlowTurnover     []UnifiedSKUFocusCardV1 `json:"slow_turnover,omitempty"`
	KeySKUs          []UnifiedKeySKUCardV1   `json:"key_skus"`
}

// UnifiedActionV1 is one seller-facing action in the unified AI output.
type UnifiedActionV1 struct {
	AnalysisActionID string `json:"analysis_action_id"`

	Type     string `json:"type"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	// ExecutionTier is required|recommended (schema v6+).
	ExecutionTier string `json:"execution_tier,omitempty"`

	Title           string `json:"title"`
	Summary         string `json:"summary"`
	Problem         string `json:"problem"`
	WhyNow          string `json:"why_now"`
	OperationalGoal string `json:"operational_goal"`
	ExpectedEffect  string `json:"expected_effect"`

	Scope UnifiedActionScopeV1 `json:"scope"`

	Steps                []UnifiedActionStepV1          `json:"steps"`
	CompletionCriteria   string                         `json:"completion_criteria"`
	CompletionConditions []UnifiedCompletionConditionV1 `json:"completion_conditions,omitempty"`

	EvidenceSummary string `json:"evidence_summary"`
	// EvidenceFactIDs lists the Pack/facts fact_ids that ground this action.
	// Each action must carry at least one fact_id that exists in the Pack
	// facts registry (schema v8+). This is first-class evidence, not a
	// legacy candidate id.
	EvidenceFactIDs []string `json:"evidence_fact_ids"`
	// SourceSignalIDs lists the diagnostic signal_ids that motivated this
	// action (optional cross-reference into what_changed / seller_pulse).
	SourceSignalIDs []string `json:"source_signal_ids"`
	Limitations     []string `json:"limitations,omitempty"`
	Confidence      string   `json:"confidence"`
}

type UnifiedActionScopeV1 struct {
	ScopeType string   `json:"scope_type"`
	EntityIDs []string `json:"entity_ids"`
}

type UnifiedActionStepV1 struct {
	Order       int      `json:"order"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	EntityIDs   []string `json:"entity_ids,omitempty"`
}

type UnifiedCompletionConditionV1 struct {
	ConditionKey string `json:"condition_key"`
	Type         string `json:"type"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	MetricField  string `json:"metric_field,omitempty"`
	// BaselineFactID references the Pack/facts fact_id whose value is the
	// baseline the completion condition is measured against. It must NOT be
	// conflated with ConditionKey (schema v8+).
	BaselineFactID string   `json:"baseline_fact_id,omitempty"`
	BaselineValue  *float64 `json:"baseline_value,omitempty"`
	Operator       string   `json:"operator,omitempty"`
	TargetValue    *float64 `json:"target_value,omitempty"`
	PeriodDays     *int     `json:"period_days,omitempty"`
}
